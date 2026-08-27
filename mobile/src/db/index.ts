import { schema } from './schema';

type DbStatus = 'OK' | 'ERROR';

let initPromise: Promise<DbStatus> | null = null;
let database: unknown = null;
let _dbStatus: DbStatus = 'OK';

async function tryInitWatermelon(): Promise<unknown> {
  const isJest =
    typeof process !== 'undefined' &&
    !!(process.env.JEST_WORKER_ID || process.env.NODE_ENV === 'test');
  const isNodeNoWindow = typeof window === 'undefined';
  if (isJest || isNodeNoWindow) {
    return { _mock: true, schema };
  }
  const { Database } = await import('@nozbe/watermelondb');
  try {
    const SQLiteAdapter = (await import('@nozbe/watermelondb/adapters/sqlite')).default;
    const adapter = new (SQLiteAdapter as unknown as new (o: unknown) => unknown)({
      schema,
      dbName: 'fleet',
      jsi: true,
      onSetUpError: (error: unknown) => {
        throw error;
      },
    });
    const db = new (Database as unknown as new (o: unknown) => unknown)({
      adapter: adapter as never,
      modelClasses: [],
      actionsEnabled: true,
    });
    return db;
  } catch {
    const LokiJSAdapter = (await import('@nozbe/watermelondb/adapters/lokijs')).default;
    const adapter = new (LokiJSAdapter as unknown as new (o: unknown) => unknown)({
      schema,
      useWebWorker: false,
      useIncrementalIndexedDB: true,
      dbName: 'fleet',
    });
    const { Database: Db2 } = await import('@nozbe/watermelondb');
    const db = new (Db2 as unknown as new (o: unknown) => unknown)({
      adapter: adapter as never,
      modelClasses: [],
      actionsEnabled: true,
    });
    return db;
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
