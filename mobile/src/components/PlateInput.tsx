import React, { useState } from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';
import { isValidPlate, normalizePlate } from '../lib/plate';

type PlateInputProps = {
  onConnect: (plate: string) => void;
};

export function PlateInput({ onConnect }: PlateInputProps) {
  const [plate, setPlate] = useState('');
  const valid = isValidPlate(plate);

  return (
    <View>
      <TextInput
        testID="plate-input"
        value={plate}
        onChangeText={(t) => setPlate(normalizePlate(t))}
        placeholder="ACF356"
        maxLength={6}
        autoCapitalize="characters"
        accessibilityLabel="Plate"
      />
      {plate.length > 0 && !valid ? <Text testID="plate-hint">3 letras + 3 números</Text> : null}
      <Pressable
        testID="connect-btn"
        disabled={!valid}
        hitSlop={{ top: 10, bottom: 10, left: 10, right: 10 }}
        style={{
          backgroundColor: valid ? '#86efac' : '#e5e7eb',
          minHeight: 44,
          justifyContent: 'center',
          alignItems: 'center',
          padding: 12,
          borderRadius: 8,
        }}
        onPress={() => {
          if (!valid) return;
          onConnect(plate);
        }}
      >
        <Text>Connect</Text>
      </Pressable>
    </View>
  );
}
