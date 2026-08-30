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
import { getTelemetryPort, getIntervalPort } from './store/ports';
import { telemetryPort } from './db/telemetryPort';
import { intervalRegistry } from './store/intervalRegistry';

if (!getTelemetryPort()) injectTelemetryPort(telemetryPort);
if (!getIntervalPort())
  injectIntervalPort({
    register: (id: number) => intervalRegistry.register(id),
    clear: (id: number) => intervalRegistry.clear(id),
    clearAll: () => intervalRegistry.clearAll(),
  });

export async function getPendingCountSafe(): Promise<number> {
  const port = getTelemetryPort();
  if (port) return port.countPending();
  const mod = await import('./db/telemetry');
  return mod.countPending();
}

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
  const pendingRef = React.useRef(pending);
  React.useEffect(() => {
    pendingRef.current = pending;
  }, [pending]);

  useEffect(() => {
    const cur = useAppStore.getState().db;
    if (cur === 'OK') return;
    let cancelled = false;
    const run = async () => {
      try {
        const t0 = Date.now();
        const status = await initDatabase();
        if (cancelled) return;
        const ms = Date.now() - t0;
        if (ms > 1000 && ms < 5000) {
          console.warn(`[db] init slow ${ms}ms`);
        }
        const curNow = useAppStore.getState().db;
        if (curNow !== status) useAppStore.setState({ db: status });
      } catch {
        if (cancelled) return;
        const curNow = useAppStore.getState().db;
        if (curNow !== 'ERROR') useAppStore.setState({ db: 'ERROR' });
      }
    };
    void run();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const c = await getPendingCountSafe();
        if (cancelled) return;
        if (pendingRef.current === c) return;
        pendingRef.current = c;
        setPending(c);
      } catch {}
    };
    void load();
    const id = setInterval(() => {
      void load().catch(() => {});
    }, 2000);
    const port = getIntervalPort();
    if (port) port.register(id as unknown as number);
    return () => {
      cancelled = true;
      const p = getIntervalPort();
      if (p) p.clear(id as unknown as number);
      else clearInterval(id);
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
        const c = await getPendingCountSafe();
        if (pendingRef.current !== c) {
          pendingRef.current = c;
          setPending(c);
        }
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
            hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
            style={{
              backgroundColor: '#f9a8d4',
              padding: 12,
              borderRadius: 8,
              marginTop: 8,
              opacity: isDisconnecting ? 0.6 : 1,
              minHeight: 44,
              justifyContent: 'center',
              alignItems: 'center',
            }}
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
    fontFamily: 'Sketch',
  },
  pending: {
    marginTop: 12,
    color: '#6b7280',
  },
});
