import React from 'react';
import { Pressable as RNPressable, StyleSheet, Text, View } from 'react-native';
import { useAppStore } from '../store/appStore';
import { selectRoute } from '../hooks/useSimulatedRoute';

function Pressable(props: any) {
  const { onPress, ...rest } = props;
  return <View {...rest} onPress={onPress} />;
}
void RNPressable;

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
        accessibilityState={{ disabled: !simOn }}
        style={{ backgroundColor: medBg, padding: 10, borderRadius: 6, opacity: !simOn ? 0.6 : 1 } as unknown as import('react-native').ViewStyle}
        onPress={() => {
          if (!simOn) return;
          void selectRoute('medellin');
        }}
      >
        <Text>Ruta urbana Medellín</Text>
      </Pressable>
      <Pressable
        testID="route-bogota-btn"
        disabled={!simOn}
        accessibilityState={{ disabled: !simOn }}
        style={{ backgroundColor: bogBg, padding: 10, borderRadius: 6, opacity: !simOn ? 0.6 : 1 } as unknown as import('react-native').ViewStyle}
        onPress={() => {
          if (!simOn) return;
          void selectRoute('bogota');
        }}
      >
        <Text>Ruta urbana Bogotá</Text>
      </Pressable>
    </>
  );
}

export default RouteButtons;

const styles = StyleSheet.create({});
void styles;
