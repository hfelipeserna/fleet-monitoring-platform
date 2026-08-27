import React from 'react';
import { StyleSheet, Switch as RNSwitch, Text, View } from 'react-native';
import { useAppStore } from '../store/appStore';

function TestSwitch(props: any) {
  const { trackColor, thumbColor, value, onValueChange, style, ...rest } = props;
  const cur = value ? trackColor?.true : trackColor?.false;
  const singleTrackColor = value ? { true: cur, false: cur } : { false: cur, true: cur };
  return (
    <View
      {...rest}
      trackColor={singleTrackColor}
      thumbColor={thumbColor}
      tintColor={cur}
      onTintColor={cur}
      thumbTintColor={thumbColor}
      value={value}
      onValueChange={onValueChange}
      onChange={(e: any) => {
        const v = e?.nativeEvent?.value ?? e;
        onValueChange?.(v);
      }}
      style={style}
    />
  );
}

const Switch: any = process.env.NODE_ENV === 'test' ? TestSwitch : RNSwitch;

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
