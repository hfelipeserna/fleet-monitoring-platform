import React, { useEffect } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { StatusBar } from 'expo-status-bar';
import { PlateInput } from './components/PlateInput';
import { StatusPanel } from './components/StatusPanel';
import { useConnection } from './hooks/useConnection';
import { useNetInfo } from './hooks/useNetInfo';
import { useAppStore } from './store/appStore';
import { initDatabase } from './db';

export default function App() {
  useNetInfo();
  const { connect } = useConnection();
  const conn = useAppStore((s) => s.conn);
  const disconnect = useAppStore((s) => s.disconnect);
  const plate = useAppStore((s) => s.plate);

  useEffect(() => {
    let cancelled = false;
    const t0 = Date.now();
    initDatabase()
      .then((status) => {
        if (cancelled) return;
        const ms = Date.now() - t0;
        if (ms > 1000) {
          console.warn(`[db] init slow ${ms}ms`);
        }
        useAppStore.setState({ db: status });
      })
      .catch(() => {
        if (!cancelled) useAppStore.setState({ db: 'ERROR' });
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const handleConnect = (p: string) => {
    connect({ plate: p, lat: 0, lon: 0, speed: 0 });
  };

  const isConnected = conn === 'connected' || conn === 'error';

  return (
    <View style={styles.container}>
      <Text style={styles.title}>fleet-mobile</Text>
      {isConnected ? (
        <View style={{ width: '100%', alignItems: 'center' }}>
          <Text testID="plate-display">{plate}</Text>
          <Pressable
            testID="disconnect-btn"
            style={{ backgroundColor: '#f9a8d4', padding: 12, borderRadius: 8, marginTop: 8 }}
            onPress={disconnect}
          >
            <Text>Disconnect</Text>
          </Pressable>
        </View>
      ) : (
        <PlateInput onConnect={handleConnect} />
      )}
      <StatusPanel />
      <Text style={styles.pending}>pending 0</Text>
      <StatusBar style="auto" />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#ffffff',
    alignItems: 'center',
    justifyContent: 'center',
    padding: 16,
  },
  title: {
    fontSize: 20,
    fontWeight: '600',
    color: '#111827',
    marginBottom: 12,
  },
  pending: {
    marginTop: 12,
    color: '#6b7280',
  },
});
