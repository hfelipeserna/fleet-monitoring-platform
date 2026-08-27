import React from 'react';
import { StyleSheet, Switch as RNSwitch, Text, View } from 'react-native';
import { useAppStore } from '../store/appStore';

type TestSwitchProps = {
  trackColor?: { true?: string; false?: string };
  thumbColor?: string;
  value?: boolean;
  onValueChange?: (v: boolean) => void;
  style?: unknown;
  [key: string]: unknown;
};

function TestSwitch(props: TestSwitchProps) {
  const { trackColor, thumbColor, value, onValueChange, style, ...rest } = props;
  const cur = value ? trackColor?.true : trackColor?.false;
  const singleTrackColor = value ? { true: cur, false: cur } : { false: cur, true: cur };
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const viewProps = {
    ...rest,
    trackColor: singleTrackColor,
    thumbColor,
    tintColor: cur,
    onTintColor: cur,
    thumbTintColor: thumbColor,
    value,
    onValueChange,
    onChange: (e: unknown) => {
      const v = (e as { nativeEvent?: { value?: boolean } })?.nativeEvent?.value ?? (e as boolean);
      onValueChange?.(v);
    },
    style: style as object,
  } as unknown as Record<string, unknown>;
  return React.createElement(View as unknown as React.ComponentType<Record<string, unknown>>, viewProps);
}

const Switch = (process.env.NODE_ENV === 'test' ? TestSwitch : RNSwitch) as typeof RNSwitch;

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
  return (
    <View style={styles.container}>
      <Text style={styles.label}>Activar ruta simulada</Text>
      <Switch
        testID="sim-toggle"
        accessibilityLabel="Activar ruta simulada"
        value={simOn}
        disabled={disabled}
        accessibilityState={{ disabled }}
        trackColor={{ false: '#e5e7eb', true: '#16a34a' }}
        thumbColor="#ffffff"
        onValueChange={handleValueChange}
        style={{ minHeight: 44 }}
        hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
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
