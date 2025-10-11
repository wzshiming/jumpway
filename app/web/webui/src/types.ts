// Type definitions for Jump Way configuration

export interface Node {
  probe: string;
  lb: string[];
}

export interface Context {
  Name: string;
  Way: Node[];
}

export interface Proxy {
  Host: string;
  Port: number;
}

export interface NoProxy {
  List: string[];
  FromEnv: string[];
  FromFile: string[];
}

export interface Config {
  CurrentContext: string;
  Contexts: Context[];
  Proxy: Proxy;
  NoProxy: NoProxy;
}
