import { appSchema, tableSchema } from '@nozbe/watermelondb';

export const schema = appSchema({
  version: 1,
  tables: [
    tableSchema({
      name: 'pending_telemetry',
      columns: [
        { name: 'client_event_id', type: 'string', isIndexed: true },
        { name: 'plate', type: 'string', isIndexed: true },
        { name: 'lat', type: 'number', isOptional: true },
        { name: 'lon', type: 'number', isOptional: true },
        { name: 'speed', type: 'number' },
        { name: 'occurred_at', type: 'number' },
        { name: 'sync_status', type: 'string', isIndexed: true },
        { name: 'attempts', type: 'number' },
        { name: 'last_error', type: 'string', isOptional: true },
        { name: 'synced_at', type: 'number', isOptional: true },
      ],
    }),
  ],
});

export const migrations = {
  migrations: [],
  minVersion: 1,
  maxVersion: 1,
};

export default schema;
