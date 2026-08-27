import React, { useEffect, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import { StatusBar } from 'expo-status-bar';
import { PlateInput } from './components/PlateInput';
import { StatusPanel } from './components/StatusPanel';
import { RouteToggle } from './components/RouteToggle';
import { RouteButtons } from './components/RouteButtons';
import { useConnection } from './hooks/useConnection';
import { useNetInfo } from './hooks/useNetInfo';
import { useTelemetryGenerator } from './hooks/useTelemetryGenerator';
import { useSync } from './hooks/useSync';
import { useAppStore } from './store/appStore';
import { initDatabase } from './db';
import { injectTelemetryPort, injectIntervalPort } from './store/appStore';
import { getTelemetryPort } from './store/ports';
import { telemetryPort } from './db/telemetryPort';
import { intervalRegistry } from './store/intervalRegistry';

injectTelemetryPort(telemetryPort);
injectIntervalPort({
  register: (id: number) => intervalRegistry.register(id),
  clear: (id: number) => intervalRegistry.clear(id),
  clearAll: () => intervalRegistry.clearAll(),
});

export default function App() {
  useNetInfo();
  useTelemetryGenerator();
  useSync();
  const { connect, disconnect: hookDisconnect } = useConnection();
  const conn = useAppStore((s) => s.conn);
  const plate = useAppStore((s) => s.plate);
  const simOn = useAppStore((s) => s.simOn);
  const isDisconnecting = useAppStore((s) => s.isDisconnecting);
  const [pending, setPending] = useState(0);

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

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const port = getTelemetryPort();
        const c = port ? await port.countPending() : await (await import('./db/telemetry')).countPending();
        if (!cancelled) setPending(c);
      } catch {}
    };
    load();
    const id = setInterval(load, 2000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [conn, plate]);

  const handleConnect = (p: string) => {
    connect({ plate: p, lat: 0, lon: 0, speed: 0 });
  };

  const isConnected = conn === 'connected' || conn === 'error';

  const handleDisconnect = async () => {
    if (isDisconnecting) return;
    try {
      await hookDisconnect();
    } finally {
      try {
        const port = getTelemetryPort();
        const c = port ? await port.countPending() : await (await import('./db/telemetry')).countPending();
        setPending(c);
      } catch {}
    }
  };

  return (
    <View style={styles.container}>
      <Text style={styles.title}>fleet-mobile</Text>
      {isConnected ? (
        <View style={{ width: '100%', alignItems: 'center' }}>
          <Text testID="plate-display">{plate}</Text>
          <Pressable
            testID="disconnect-btn"
            disabled={isDisconnecting}
            style={{ backgroundColor: '#f9a8d4', padding: 12, borderRadius: 8, marginTop: 8, opacity: isDisconnecting ? 0.6 : 1 }}
            onPress={handleDisconnect}
          >
            <Text>Disconnect</Text>
          </Pressable>
        </View>
      ) : (
        <PlateInput onConnect={handleConnect} />
      )}
      <StatusPanel />
      <RouteToggle />
      <Text style={{ marginTop: 8 }}>{`Activar ruta simulada ${simOn ? 'ON' : 'OFF'}`}</Text>
      <View style={{ flexDirection: 'row', gap: 8, marginTop: 8 }}>
        <RouteButtons />
      </View>
      <Text style={styles.pending}>{`pending ${pending}`}</Text>
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
