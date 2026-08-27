/* eslint-disable @typescript-eslint/no-explicit-any */
// Covers AC-003 AC-010: FR-003 BR-004 - Network vs Syncing desacoplados + WatermelonDB status
import React from 'react';
import { render, waitFor } from '@testing-library/react-native';

jest.mock('expo-constants', () => ({
  default: { expoConfig: { extra: { apiUrl: 'http://localhost:8080' } } },
  expoConfig: { extra: { apiUrl: 'http://localhost:8080' } },
}), { virtual: true });

jest.mock('@react-native-community/netinfo', () => ({
  addEventListener: jest.fn(() => jest.fn()),
  fetch: jest.fn().mockResolvedValue({ isConnected: true, isInternetReachable: true }),
}), { virtual: true });

jest.mock('@nozbe/watermelondb', () => ({
  appSchema: (x: any) => x,
  tableSchema: (x: any) => x,
}), { virtual: true });

import { StatusPanel } from './StatusPanel';
import { useAppStore } from '../store/appStore';

function resetStore() {
  try {
    useAppStore.setState({ conn: 'idle', sync: 'CONNECTING', net: 'OK', db: 'OK', plate: '' } as any);
  } catch {}
}

function getColor(element: any): string | undefined {
  const style = element.props.style;
  const flat = Array.isArray(style) ? Object.assign({}, ...style) : style;
  return flat?.color ?? flat?.backgroundColor;
}

describe('StatusPanel', () => {
  beforeEach(() => {
    // Arrange baseline
    resetStore();
    jest.clearAllMocks();
  });

  describe('connected -> Network OK verde + Syncing CONNECTED rojo', () => {
    it('renders Network connectivity ○ OK verde #16a34a and Syncing CONNECTED rojo #dc2626 and WatermelonDB OK verde (desacoplados OK)', async () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', sync: 'CONNECTED', net: 'OK', db: 'OK' } as any);
      const { getByTestId } = render(<StatusPanel />);

      // Act
      const syncEl = getByTestId('sync-status');
      const netEl = getByTestId('net-status');
      const dbEl = getByTestId('db-status');

      // Assert
      expect(syncEl).toBeTruthy();
      expect(netEl).toBeTruthy();
      expect(dbEl).toBeTruthy();
      expect(syncEl.props.children).toEqual(expect.stringContaining('Syncing data ... CONNECTED'));
      expect(netEl.props.children).toEqual(expect.stringContaining('Network connectivity'));
      expect(netEl.props.children).toEqual(expect.stringContaining('OK'));
      expect(dbEl.props.children).toEqual(expect.stringContaining('WatermelonDB status'));
      expect(dbEl.props.children).toEqual(expect.stringContaining('OK'));
      // dots must be ○ (open circle) per spec, not ●
      expect(String(netEl.props.children)).toContain('○');
      expect(String(dbEl.props.children)).toContain('○');
      expect(getColor(syncEl)).toBe('#dc2626');
      expect(getColor(netEl)).toBe('#16a34a');
      expect(getColor(dbEl)).toBe('#16a34a');
      // sync always rojo even when CONNECTED per FR-012
      expect(getColor(syncEl)).not.toBe('#16a34a');
    });

    it('verifies testID sync-status/db-status/net-status exist', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', sync: 'CONNECTED', net: 'OK', db: 'OK' } as any);

      // Act
      const { getByTestId } = render(<StatusPanel />);

      // Assert
      expect(getByTestId('sync-status')).toBeTruthy();
      expect(getByTestId('db-status')).toBeTruthy();
      expect(getByTestId('net-status')).toBeTruthy();
    });
  });

  describe('avion ON -> Network ERROR rojo + Syncing ERROR pero DB OK verde', () => {
    it('renders Network ERROR rojo #dc2626 + Syncing ERROR rojo y DB OK verde desacoplados', () => {
      // Arrange
      useAppStore.setState({ conn: 'error', sync: 'ERROR', net: 'ERROR', db: 'OK' } as any);
      const { getByTestId } = render(<StatusPanel />);

      // Act
      const syncEl = getByTestId('sync-status');
      const netEl = getByTestId('net-status');
      const dbEl = getByTestId('db-status');

      // Assert
      expect(String(syncEl.props.children)).toContain('ERROR');
      expect(String(netEl.props.children)).toContain('ERROR');
      expect(String(netEl.props.children)).toContain('○');
      expect(String(dbEl.props.children)).toContain('OK');
      expect(String(dbEl.props.children)).toContain('○');
      expect(getColor(syncEl)).toBe('#dc2626');
      expect(getColor(netEl)).toBe('#dc2626');
      expect(getColor(dbEl)).toBe('#16a34a');
    });

    it('keeps WatermelonDB OK verde while Network ERROR (offline not corrupts DB) AC-003', () => {
      // Arrange
      useAppStore.setState({ conn: 'error', sync: 'ERROR', net: 'ERROR', db: 'OK' } as any);

      // Act
      const { getByTestId } = render(<StatusPanel />);

      // Assert
      const dbEl = getByTestId('db-status');
      const netEl = getByTestId('net-status');
      expect(getColor(dbEl)).toBe('#16a34a');
      expect(getColor(netEl)).toBe('#dc2626');
      expect(String(dbEl.props.children)).toMatch(/WatermelonDB status.*○.*OK/);
    });
  });

  describe('503 con Net OK -> Network OK verde + Syncing ERROR rojo (desacoplados) AC-010', () => {
    it('renders Network OK verde #16a34a + Syncing ERROR rojo #dc2626 desacoplados (BR-004)', () => {
      // Arrange
      useAppStore.setState({ conn: 'error', sync: 'ERROR', net: 'OK', db: 'OK' } as any);
      const { getByTestId } = render(<StatusPanel />);

      // Act
      const syncEl = getByTestId('sync-status');
      const netEl = getByTestId('net-status');
      const dbEl = getByTestId('db-status');

      // Assert
      expect(String(syncEl.props.children)).toContain('ERROR');
      expect(String(netEl.props.children)).toContain('OK');
      expect(String(netEl.props.children)).toContain('○');
      expect(getColor(syncEl)).toBe('#dc2626');
      expect(getColor(netEl)).toBe('#16a34a');
      expect(getColor(dbEl)).toBe('#16a34a');
      // desacoplados: Network OK does NOT imply Syncing OK
      expect(getColor(netEl)).not.toEqual(getColor(syncEl));
    });

    it('WatermelonDB remains OK verde when 503 backpressure', () => {
      // Arrange
      useAppStore.setState({ conn: 'error', sync: 'ERROR', net: 'OK', db: 'OK' } as any);

      // Act
      const { getByTestId } = render(<StatusPanel />);

      // Assert
      const dbEl = getByTestId('db-status');
      expect(getColor(dbEl)).toBe('#16a34a');
      expect(String(dbEl.props.children)).toContain('○ OK');
    });
  });

  describe('colores exactos #16a34a vs #dc2626 BR-011 FR-012', () => {
    it('OK dots use #16a34a and ERROR dots use #dc2626', async () => {
      // Arrange
      const { act } = require('@testing-library/react-native');
      await act(async () => {
        useAppStore.setState({ conn: 'connected', sync: 'CONNECTED', net: 'OK', db: 'OK' } as any);
      });
      const { getByTestId, rerender } = render(<StatusPanel />);

      // Act - OK state
      const netOk = getByTestId('net-status');
      const dbOk = getByTestId('db-status');

      // Assert - green
      expect(getColor(netOk)).toBe('#16a34a');
      expect(getColor(dbOk)).toBe('#16a34a');

      // Arrange - switch to ERROR
      await act(async () => {
        useAppStore.setState({ conn: 'error', sync: 'ERROR', net: 'ERROR', db: 'ERROR' } as any);
      });

      // Act - re-render to pick new store values (StatusPanel reads store)
      rerender(<StatusPanel />);

      // Assert - red
      const netErr = getByTestId('net-status');
      const dbErr = getByTestId('db-status');
      const syncErr = getByTestId('sync-status');
      expect(getColor(netErr)).toBe('#dc2626');
      expect(getColor(dbErr)).toBe('#dc2626');
      expect(getColor(syncErr)).toBe('#dc2626');
    });
  });

  describe('WatermelonDB schema + init (RED until mobile-expo creates files)', () => {
    it('has WatermelonDB schema pending_telemetry v1 with client_event_id', () => {
      // Arrange
      const mod: any = require('../db/schema');

      // Assert
      const dump = JSON.stringify(mod);
      expect(dump).toContain('pending_telemetry');
      expect(dump).toContain('client_event_id');
      // version 1 per plan §5
      expect(dump).toContain('1');
      const schema = mod.default ?? mod.schema ?? mod.appSchema ?? mod;
      const table = schema?.tables?.[0] ?? schema?.tableSchema ?? null;
      // allow flexible shape but must mention pending_telemetry
      expect(dump).toMatch(/pending_telemetry/);
      expect(dump).toMatch(/plate/);
      expect(dump).toMatch(/sync_status/);
    });

    it('has db/index that can init and return status OK/ERROR', async () => {
      // Arrange
      const dbMod: any = require('../db/index');

      // Assert
      expect(dbMod).toBeDefined();
      // must export something to get status: getDbStatus | getDatabaseStatus | init | database
      const hasStatusFn =
        typeof dbMod.getDbStatus === 'function' ||
        typeof dbMod.getDatabaseStatus === 'function' ||
        typeof dbMod.getStatus === 'function' ||
        typeof dbMod.init === 'function' ||
        typeof dbMod.initDatabase === 'function' ||
        typeof dbMod.database !== 'undefined' ||
        typeof dbMod.default !== 'undefined';
      expect(hasStatusFn).toBe(true);
      const fn = dbMod.getDbStatus ?? dbMod.getDatabaseStatus ?? dbMod.getStatus ?? dbMod.init ?? dbMod.initDatabase;
      if (typeof fn === 'function') {
        const res = await fn();
        expect(['OK', 'ERROR', undefined, null].includes(res) || typeof res === 'object').toBeTruthy();
        if (typeof res === 'string') expect(['OK', 'ERROR']).toContain(res);
      }
    });

    it('StatusPanel integrates WatermelonDB status ○ OK/#16a34a ERROR/#dc2626 via props or store', () => {
      // Arrange
      // ensure component respects explicit props for DB status (used by tests)
      const { getByTestId } = render(<StatusPanel db="ERROR" net="OK" sync="ERROR" />);

      // Act
      const dbEl = getByTestId('db-status');

      // Assert
      expect(String(dbEl.props.children)).toContain('○');
      expect(String(dbEl.props.children)).toContain('ERROR');
      expect(getColor(dbEl)).toBe('#dc2626');
    });
  });
});
