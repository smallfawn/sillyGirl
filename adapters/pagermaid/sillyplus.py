import contextlib
import io
import json
import os
import urllib.parse
from asyncio import sleep

from pagermaid import log
from pagermaid.enums import Message
from pagermaid.hook import Hook
from pagermaid.listener import listener
from pagermaid.services import bot
from pagermaid.single_utils import sqlite
from pagermaid.utils import pip_install

pip_install("aiohttp")

import aiohttp

uri = os.getenv("SILLYGIRL_PAGERMAID_WS", "${rws()}").strip()


def with_user_id(value: str, user_id: str) -> str:
    if not value:
        return value
    parsed = urllib.parse.urlsplit(value)
    query = urllib.parse.parse_qsl(parsed.query, keep_blank_values=True)
    query = [(key, val) for key, val in query if key != "user_id"]
    query.append(("user_id", str(user_id)))
    return urllib.parse.urlunsplit(parsed._replace(query=urllib.parse.urlencode(query)))


class WebSocket:
    def __init__(self):
        self.uri = uri
        self.loop = bot.loop
        self.client = aiohttp.ClientSession(loop=self.loop)
        self.need_stop = False
        self.ws = None
        self.connection = None
        self.whitelist = []

    @staticmethod
    def database_have_uri():
        return uri != ""

    def restore_uri(self):
        self.uri = uri

    async def set_uri(self, uri):
        await self.disconnect()
        self.uri = uri
        sqlite["websocket_uri"] = uri

    def is_connected(self):
        return self.connection is not None

    async def connect(self):
        if self.is_connected():
            await self.disconnect()
        if self.uri:
            self.ws = self.client.ws_connect(
                with_user_id(self.uri, bot.me.id),
                autoclose=False,
                autoping=False,
                timeout=5,
            )
            self.connection = await self.ws._coro

    async def disconnect(self):
        if self.connection:
            with contextlib.suppress(Exception):
                await self.connection.close()
            self.ws = None
            self.connection = None

    async def keep_alive(self):
        while True:
            try:
                if not self.uri:
                    await sleep(1)
                    continue
                await self.connect()
            except Exception:
                await sleep(1)
                continue
            await self.get()
            await sleep(1)
            if self.need_stop:
                await self.disconnect()
                self.need_stop = False
                break

    async def get(self):
        ws_ = self.connection
        if not ws_:
            return
        while True:
            msg = await ws_.receive()
            if msg.type == aiohttp.WSMsgType.TEXT:
                bot.loop.create_task(self.process_message(msg.data))
            elif msg.type == aiohttp.WSMsgType.PING:
                await ws_.pong()
            elif msg.type == aiohttp.WSMsgType.BINARY:
                pass
            elif msg.type != aiohttp.WSMsgType.PONG:
                if msg.type == aiohttp.WSMsgType.CLOSE:
                    await ws_.close()
                elif msg.type == aiohttp.WSMsgType.ERROR:
                    print(f"Error during receive {ws_.exception()}")
                break
        self.ws = None
        self.connection = None

    async def push(self, msg):
        if self.is_connected():
            await self.connection.send_str(msg)

    @staticmethod
    async def process_message(text: str):
        try:
            data = json.loads(text)
        except Exception:
            return
        action = data.get("action")
        if not action:
            return
        if action == "set_whitelist":
            ws.whitelist = data.get("data") or []
            return
        echo = data.get("echo", "")
        action_data = data.get("data") or {}
        bot_action = getattr(bot, action, None)
        if not bot_action:
            return
        if action == "send_document":
            action_data['document'] = io.BytesIO(str.encode(action_data['document']))

        if bot_action and isinstance(action_data, dict):
            message = await bot_action(**action_data)
            message = str(message.__str__())
            if message.strip().startswith("{"):
                message = message.replace("{", '{"echo": "' + str(echo) + '",', 1)
            else:
                message = json.dumps({"echo": str(echo), "message": message}, ensure_ascii=False)
            await ws.push(message)


ws = WebSocket()


@Hook.on_startup()
async def connect_ws():
    try:
        # await ws.connect()
        bot.loop.create_task(ws.keep_alive())
    except Exception as e:
        await log(f"[ws] Connection failed: {e}")


@listener(incoming=True, outgoing=False, ignore_edited=True)
async def websocket_push(message: Message):
    with contextlib.suppress(Exception):
        await ws.push(message.__str__())


@listener(command="sillyGirl", description="sillyGirl Connect")
async def websocket_to_connect(message: Message):
    if ws.is_connected():
        return await message.edit("傻+ 已连接")
    else:
        return await message.edit("傻+ 已离线")

