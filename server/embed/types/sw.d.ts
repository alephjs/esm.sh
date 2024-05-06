export interface ArchiveEntry {
  name: string;
  type: string;
  lastModified: number;
  offset: number;
  size: number;
}

export interface Plugin {
  (sw: Omit<SW, "fire" | "listen">): void;
  displayName?: string;
}

export interface FireOptions {
  main?: string;
  swModule?: string;
  swUpdateViaCache?: ServiceWorkerUpdateViaCache;
}

export interface SW {
  use(...plugins: readonly Plugin[]): this;
  onFetch(handler: (event: FetchEvent) => void): this;
  onFire(handler: (sw: ServiceWorker) => void): this;
  onUpdateFound: () => void;
  waitUntil(...promises: readonly Promise<any>[]): this;
  fire(options?: FireOptions): Promise<ServiceWorker>;
  listen(): void;
}

export const sw: SW;
export default sw;
