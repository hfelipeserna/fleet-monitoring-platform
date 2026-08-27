import React from 'react';
import { Pressable, Text } from 'react-native';
import { useAppStore } from '../store/appStore';
import { selectRoute } from '../hooks/useSimulatedRoute';

export function RouteButtons() {
  const simOn = useAppStore((s) => s.simOn);
  const selectedRoute = useAppStore((s) => s.selectedRoute);

  const medBg = !simOn ? '#e5e7eb' : selectedRoute === 'medellin' ? '#86efac' : '#93c5fd';
  const bogBg = !simOn ? '#e5e7eb' : selectedRoute === 'bogota' ? '#86efac' : '#93c5fd';

  return (
    <>
      <Pressable
        testID="route-medellin-btn"
        disabled={!simOn}
        accessibilityRole="button"
        accessibilityState={{ disabled: !simOn }}
        hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
        style={{
          backgroundColor: medBg,
          padding: 10,
          borderRadius: 6,
          opacity: !simOn ? 0.6 : 1,
          minHeight: 44,
          justifyContent: 'center',
          alignItems: 'center',
        }}
        onPress={() => {
          if (!simOn) return;
          selectRoute('medellin');
        }}
      >
        <Text>Ruta urbana Medellín</Text>
      </Pressable>
      <Pressable
        testID="route-bogota-btn"
        disabled={!simOn}
        accessibilityRole="button"
        accessibilityState={{ disabled: !simOn }}
        hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
        style={{
          backgroundColor: bogBg,
          padding: 10,
          borderRadius: 6,
          opacity: !simOn ? 0.6 : 1,
          minHeight: 44,
          justifyContent: 'center',
          alignItems: 'center',
        }}
        onPress={() => {
          if (!simOn) return;
          selectRoute('bogota');
        }}
      >
        <Text>Ruta urbana Bogotá</Text>
      </Pressable>
    </>
  );
}

export default RouteButtons;
