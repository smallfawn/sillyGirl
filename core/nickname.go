package core

import "time"

type Nickname struct {
	ID       string   `json:"i"`
	Group    bool     `json:"g"`
	Unix     int      `json:"u"`
	Value    string   `json:"v"`
	Platform string   `json:"p"`
	BotsID   []string `json:"bs"`
}

var nickname = MakeBucket("nickname")

type NicklabeL struct {
	Label    string `json:"label"`
	Value    string `json:"value"`
	Platform string `json:"platform"`
	ChatName string `json:"chat_name"`
}

func CreateNickName(nick *Nickname) {
	nick.Unix = int(time.Now().Unix())
	nickname.Create(nick)
}
