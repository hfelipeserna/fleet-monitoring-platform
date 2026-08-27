import React from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { StatusBar } from 'expo-status-bar';
import { PlateInput } from './components/PlateInput';

export default function App() {
  return (
    <View style={styles.container}>
      <Text style={styles.title}>fleet-mobile</Text>
      <PlateInput onConnect={() => {}} />
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
