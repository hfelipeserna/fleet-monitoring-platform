import { schema } from './schema';
import Constants from 'expo-constants';

type DbStatus = 'OK' | 'ERROR';

let initPromise: Promise<DbStatus> | null = null;
let database: unknown = null;
let _dbStatus: DbStatus = 'OK';

function isExpoGo(): boolean {
  try {
    const c = Constants as unknown as Record<string, unknown>;
    const ownership = (c.appOwnership as string | undefined) ?? (c.executionEnvironment as string | undefined);
    if (ownership === 'expo' || ownership === 'storeClient') return true;
    const execEnv = (c as Record<string, unknown>).executionEnvironment as string | undefined;
    if (execEnv === 'storeClient') return true;
    const expoConfig = (c.expoConfig as Record<string, unknown> | undefined) ?? (c as Record<string, unknown>).manifest as Record<string, unknown> | undefined;
    if (expoConfig && (expoConfig.executionEnvironment as string | undefined) === 'storeClient') return true;
    const def = (c.default as Record<string, unknown> | undefined)?.expoConfig as Record<string, unknown> | undefined;
    if (def && (def.executionEnvironment as string | undefined) === 'storeClient') return true;
    if ((c as Record<string, unknown>).appOwnership === 'expo') return true;
  } catch {}
  return false;
}

async function createLokiDatabase(): Promise<unknown> {
  const LokiJSAdapter = (await import('@nozbe/watermelondb/adapters/lokijs')).default;
  const adapter = new (LokiJSAdapter as unknown as new (o: unknown) => unknown)({
    schema,
    useWebWorker: false,
    useIncrementalIndexedDB: true,
    dbName: 'fleet',
  });
  const { Database } = await import('@nozbe/watermelondb');
  const db = new (Database as unknown as new (o: unknown) => unknown)({
    adapter: adapter as never,
    modelClasses: [],
    actionsEnabled: true,
  });
  return db;
}

async function tryInitWatermelon(): Promise<unknown> {
  const isJest =
    typeof process !== 'undefined' &&
    !!(process.env.JEST_WORKER_ID || process.env.NODE_ENV === 'test');
  const isNodeNoWindow = typeof window === 'undefined';
  if (isJest || isNodeNoWindow) {
    return { _mock: true, schema };
  }
  if (isExpoGo()) {
    try {
      return await createLokiDatabase();
    } catch (e) {
      console.warn('[db] LokiJS init failed in Expo Go, fallback mock', e);
      return { _mock: true, schema };
    }
  }
  const { Database } = await import('@nozbe/watermelondb');
  try {
    const SQLiteAdapter = (await import('@nozbe/watermelondb/adapters/sqlite')).default;
    const adapter = new (SQLiteAdapter as unknown as new (o: unknown) => unknown)({
      schema,
      dbName: 'fleet',
      jsi: true,
      onSetUpError: (error: unknown) => {
        console.warn('[db] SQLite onSetUpError', error);
      },
    });
    const db = new (Database as unknown as new (o: unknown) => unknown)({
      adapter: adapter as never,
      modelClasses: [],
      actionsEnabled: true,
    });
    return db;
  } catch (e) {
    console.warn('[db] SQLite JSI failed, fallback LokiJS', e);
    try {
      return await createLokiDatabase();
    } catch (e2) {
      console.warn('[db] LokiJS fallback failed, using mock', e2);
      return { _mock: true, schema };
    }
  }
}

export async function initDatabase(): Promise<DbStatus> {
  if (initPromise) return initPromise;
  initPromise = (async (): Promise<DbStatus> => {
    try {
      const db = await tryInitWatermelon();
      database = db;
      _dbStatus = 'OK';
      return 'OK' as DbStatus;
    } catch (err) {
      database = null;
      _dbStatus = 'ERROR';
      throw err;
    }
  })().catch(() => {
    _dbStatus = 'ERROR';
    database = null;
    return 'ERROR' as DbStatus;
  }) as Promise<DbStatus>;
  return initPromise;
}

export async function getDbStatus(): Promise<DbStatus> {
  if (initPromise) return initPromise;
  return initDatabase();
}

export async function getDatabaseStatus(): Promise<DbStatus> {
  return getDbStatus();
}

export async function getStatus(): Promise<DbStatus> {
  return getDbStatus();
}

export function getDatabase(): unknown {
  return database;
}

export { database, schema };
