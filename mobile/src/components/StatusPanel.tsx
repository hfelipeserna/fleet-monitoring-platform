import React from 'react';
import { Text, View } from 'react-native';
import { useAppStore } from '../store/appStore';

type StatusPanelProps = {
  sync?: string;
  db?: string;
  net?: string;
};

export function StatusPanel(props: StatusPanelProps) {
  const storeSync = useAppStore((s) => s.sync);
  const storeDb = useAppStore((s) => s.db);
  const storeNet = useAppStore((s) => s.net);

  const sync = props.sync ?? storeSync;
  const db = props.db ?? storeDb;
  const net = props.net ?? storeNet;

  return (
    <View>
      <Text testID="sync-status" style={{ color: '#dc2626', fontFamily: 'Sketch' }}>
        {`Syncing data ... ${sync}`}
      </Text>
      <Text
        testID="db-status"
        style={{ color: db === 'OK' ? '#16a34a' : '#dc2626', fontFamily: 'Sketch' }}
      >
        {`WatermelonDB status ○ ${db}`}
      </Text>
      <Text
        testID="net-status"
        style={{ color: net === 'OK' ? '#16a34a' : '#dc2626', fontFamily: 'Sketch' }}
      >
        {`Network connectivity ○ ${net}`}
      </Text>
    </View>
  );
}
