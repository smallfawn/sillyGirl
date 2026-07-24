export type ApiResult<T = unknown> = {
  success?: boolean;
  data?: T;
  page?: number;
  total?: number;
  status?: string;
  currentAuthority?: string;
  errorMessage?: string;
  [key: string]: unknown;
};

export type CurrentUser = {
  name?: string;
  avatar?: string;
  plugins?: Array<{ path: string; name: string; create_at?: string; type?: string; file?: string; plugin?: string }>;
  adapters?: AdapterStatus[];
  integrations?: Record<string, IntegrationStatus>;
  version?: VersionInfo;
};

export type AdapterStatus = {
  platform: string;
  label: string;
  online: boolean;
  bots_id?: string[];
  count?: number;
};

export type IntegrationStatus = {
  label: string;
  count: number;
  online_count: number;
  online: boolean;
};

export type VersionInfo = {
  local?: string;
  remote?: string;
  source?: string;
  repository?: string;
};

export type PluginInfo = {
  id: string;
  title: string;
  suffix?: string;
  desc?: string;
  version?: string;
  author?: string;
  icon?: string;
  status?: number;
  current_version?: string;
  latest_version?: string;
  update_content?: string;
  disable?: boolean;
  running?: boolean;
  debug?: boolean;
  public?: boolean;
  module?: boolean;
  on_start?: boolean;
  create_at?: string;
  class?: string;
  organization?: string;
  address?: string;
  messages?: unknown;
};

export type Reply = {
  id?: number;
  index?: number;
  nickname?: string;
  number?: string;
  priority?: number;
  keyword?: string;
  value?: string;
  created_at?: number;
  platforms?: string[];
};

export type Master = {
  id?: number;
  platform?: string;
  nickname?: string;
  number?: string;
  unix?: number;
};

export type CarryGroup = {
  id?: number;
  chat_id: string;
  remark?: string;
  platform?: string;
  created_at?: number;
  bots_id?: string[];
  scripts?: string[];
};

export type Task = {
  id?: number;
  task_id?: string;
  title?: string;
  schedule?: string;
  senders?: Array<{ chat_id?: string; user_id?: string; platform?: string; bot_id?: string }>;
  command?: string;
  scripts?: string[];
  created_at?: number;
  remark?: string;
  enable?: boolean;
};

export type QinglongPanel = {
  id?: string;
  name?: string;
  address: string;
  client_id: string;
  client_secret: string;
  created_at?: number;
  updated_at?: number;
  last_checked_at?: number;
  status?: string;
  message?: string;
};

export type SmallcatPanel = {
  id?: string;
  name?: string;
  address: string;
  api_auth: string;
  created_at?: number;
  updated_at?: number;
  last_checked_at?: number;
  status?: string;
  message?: string;
};

export type DaidaiPanel = {
  id?: string;
  name?: string;
  address: string;
  app_key: string;
  app_secret: string;
  created_at?: number;
  updated_at?: number;
  last_checked_at?: number;
  status?: string;
  message?: string;
};
