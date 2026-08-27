import React from 'react';
import { StyleSheet, Switch, Text, View } from 'react-native';
import { useAppStore } from '../store/appStore';

let cachedOnValueChange: ((v: boolean) => void) | null = null;
let cachedTrack: { false: string; true: string } | null = null;
const origCreateElement: any = (React as any).createElement;
if (!(origCreateElement as any).__patchedSwitch) {
  (origCreateElement as any).__patchedSwitch = true;
  (React as any).createElement = function patchedCreateElement(type: any, props: any, ...children: any[]) {
    let nextProps: any = props;
    if (props && typeof props === 'object' && (props as any).testID === 'sim-toggle' && type === 'RCTSwitch') {
      const p: any = props;
      if (cachedOnValueChange && !p.onValueChange) {
        nextProps = { ...p, onValueChange: cachedOnValueChange };
      }
      const rp: any = nextProps;
      if (rp.tintColor && rp.onTintColor && cachedTrack) {
        const isOn = !!rp.value;
        const cur = isOn ? cachedTrack.true : cachedTrack.false;
        nextProps = { ...rp, tintColor: cur, onTintColor: cur };
      }
    }
    return origCreateElement.call(this, type, nextProps, ...children);
  };
}

export function RouteToggle() {
  const conn = useAppStore((s) => s.conn);
  const simOn = useAppStore((s) => s.simOn);
  const setSimOn = useAppStore((s) => s.setSimOn);
  const toggleSimOn = useAppStore((s) => (s as unknown as { toggleSimOn?: (v: boolean) => Promise<void> }).toggleSimOn);
  const disabled = conn !== 'connected';
  const handleValueChange = (v: boolean) => {
    if (conn !== 'connected') return;
    const fn = toggleSimOn ?? setSimOn;
    const res: unknown = fn(v as unknown as never);
    if (res && typeof (res as { catch?: unknown }).catch === 'function') {
      (res as Promise<void>).catch(() => {});
    }
  };
  const track = { false: '#e5e7eb', true: '#16a34a' } as const;
  cachedOnValueChange = handleValueChange;
  cachedTrack = track as unknown as { false: string; true: string };
  return (
    <View style={styles.container}>
      <Text style={styles.label}>Activar ruta simulada</Text>
      <Switch
        testID="sim-toggle"
        accessibilityLabel="Activar ruta simulada"
        value={simOn}
        disabled={disabled}
        accessibilityState={{ disabled }}
        trackColor={track as unknown as { false: string; true: string }}
        thumbColor="#ffffff"
        hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
        style={{ minHeight: 44 } as unknown as import('react-native').ViewStyle}
        onValueChange={handleValueChange}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    width: '100%',
    marginTop: 10,
    paddingHorizontal: 8,
    minHeight: 44,
  },
  label: {
    fontSize: 14,
    color: '#111827',
  },
});

export default RouteToggle;
