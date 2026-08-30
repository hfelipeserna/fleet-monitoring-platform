// Covers [SPEC-005: AC-004] FR-004 FR-012 BR-005 BR-011 TS-004
// TEST-004: Disconnect rosa #f9a8d4 visible solo en connected/error, press -> abort + purga + reset idle + input "" + toggle OFF gris
// RED until App/PlateInput/store implementan disconnect purga + reset

import React from 'react';
import { render, fireEvent, waitFor, act } from '@testing-library/react-native';

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

// Mock telemetry db so we can spy on clearPending/count
const mockClearPending = jest.fn(async () => {});
const mockCountPending = jest.fn(async () => 0);
jest.mock('../db/telemetry', () => ({
  enqueue: jest.fn(async () => {}),
  clearPending: (...args: any[]) => (mockClearPending as any)(...args),
  countPending: (...args: any[]) => (mockCountPending as any)(...args),
  getPending: jest.fn(async () => []),
  markSynced: jest.fn(async () => {}),
}), { virtual: true });

const abortSpy = jest.fn();
const originalAbort = global.AbortController;

import { useAppStore } from '../store/appStore';
import App from '../App';

function resetStore() {
  try {
    useAppStore.setState({
      conn: 'idle',
      sync: 'IDLE',
      net: 'OK',
      db: 'OK',
      plate: '',
      simOn: false,
      simEnabled: false,
      selectedRoute: null,
    } as any);
  } catch {}
}

describe('Disconnect button AC-004 FR-004 BR-005 FR-012', () => {
  beforeEach(() => {
    // Arrange
    resetStore();
    jest.clearAllMocks();
    mockClearPending.mockClear();
    abortSpy.mockClear();
    // Spy AbortController abort
    (global as any).AbortController = class extends (originalAbort as any) {
      abort(...args: any[]) {
        abortSpy(...args);
        return super.abort(...args);
      }
    } as any;
    // baseline: fetch never resolves unless mocked per test
    jest.spyOn(global as any, 'fetch').mockResolvedValue({
      status: 202,
      headers: { get: jest.fn().mockReturnValue(null) } as any,
      json: async () => ({ accepted: true }),
    } as any);
  });

  afterEach(() => {
    (global as any).AbortController = originalAbort;
    jest.restoreAllMocks();
  });

  describe('visibility rosa #f9a8d4 solo en connected/error', () => {
    it('idle -> Disconnect not visible, Connect visible (PlateInput)', async () => {
      // Arrange
      useAppStore.setState({ conn: 'idle', plate: '' } as any);
      const { queryByTestId, getByTestId } = render(<App />);

      // Act
      const disconnect = queryByTestId('disconnect-btn');
      const connect = queryByTestId('connect-btn');

      // Assert
      expect(disconnect).toBeNull();
      expect(connect).toBeTruthy();
    });

    it('connected -> Disconnect visible rosa #f9a8d4', async () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', plate: 'TGY589', sync: 'CONNECTED' } as any);

      // Act
      const { getByTestId } = render(<App />);
      const btn = getByTestId('disconnect-btn');

      // Assert
      expect(btn).toBeTruthy();
      const style = btn.props.style;
      const flat = Array.isArray(style) ? Object.assign({}, ...style) : style;
      const dump = JSON.stringify(style);
      const hasPink = dump.includes('#f9a8d4') || flat?.backgroundColor === '#f9a8d4';
      expect(hasPink).toBe(true);
    });

    it('error -> Disconnect visible rosa #f9a8d4', async () => {
      // Arrange
      useAppStore.setState({ conn: 'error', plate: 'TGY589', sync: 'ERROR' } as any);

      // Act
      const { getByTestId } = render(<App />);
      const btn = getByTestId('disconnect-btn');

      // Assert
      expect(btn).toBeTruthy();
      const dump = JSON.stringify(btn.props.style);
      expect(dump.includes('#f9a8d4')).toBe(true);
    });

    it('connecting -> Disconnect not visible', async () => {
      // Arrange
      useAppStore.setState({ conn: 'connecting', plate: 'TGY589' } as any);
      const { queryByTestId } = render(<App />);

      // Act
      const btn = queryByTestId('disconnect-btn');

      // Assert
      expect(btn).toBeNull();
    });
  });

  describe('press Disconnect -> abort + purga + reset AC-004', () => {
    it('given connected 20 pending + toggle ON when Disconnect then 0 pending + idle + input "" + OFF gris + intervalo stopped', async () => {
      // Arrange
      jest.useFakeTimers();
      // Simulate 20 pending before disconnect
      let pendingCount = 20;
      mockCountPending.mockImplementation(async () => pendingCount);
      mockClearPending.mockImplementation(async () => { pendingCount = 0; });
      // Simulate store with toggle ON and interval 5s generation
      const setIntervalSpy = jest.spyOn(global as any, 'setInterval');
      const clearIntervalSpy = jest.spyOn(global as any, 'clearInterval');
      useAppStore.setState({
        conn: 'connected',
        plate: 'TGY589',
        sync: 'CONNECTED',
        simOn: true,
        simEnabled: true,
        selectedRoute: 'medellin' as any,
      } as any);
      // Simulate an interval was started (app or generator would have started it)
      const fakeInterval: any = setInterval(() => {}, 5000);
      const { getByTestId, queryByTestId } = render(<App />);
      const disconnectBtn = getByTestId('disconnect-btn');
      expect(getByTestId('plate-display').props.children).toBe('TGY589');

      // Act
      await act(async () => {
        fireEvent.press(disconnectBtn);
        await Promise.resolve();
      });
      // allow async clearPending to be awaited if store disconnect is async
      await act(async () => {
        jest.advanceTimersByTime(10);
        await Promise.resolve();
      });

      // Assert
      // 1) clearPending called (DELETE)
      expect(mockClearPending).toHaveBeenCalled();
      // 2) pending 0 via WatermelonDB mock
      expect(await mockCountPending()).toBe(0);
      // 3) store reset to idle
      expect(useAppStore.getState().conn).toBe('idle');
      expect(useAppStore.getState().plate).toBe('');
      // 4) sync reset to IDLE, simOff
      expect(useAppStore.getState().sync).toBe('IDLE');
      expect(useAppStore.getState().simOn).toBe(false);
      expect(useAppStore.getState().simEnabled).toBe(false);
      expect(useAppStore.getState().selectedRoute).toBeNull();
      // 5) interval stopped: clearInterval called at least once for 5s generator
      // The fake interval we created should be cleared by disconnect logic; at minimum clearInterval was invoked
      expect(clearIntervalSpy).toHaveBeenCalled();
      // 6) App now shows Connect disabled verde, not Disconnect, and input "" (PlateInput empty)
      expect(queryByTestId('disconnect-btn')).toBeNull();
      const connectBtn = getByTestId('connect-btn');
      expect(connectBtn).toBeTruthy();
      (expect(connectBtn) as any).toBeDisabled();
      // Connect verde disabled check: style background #86efac when enabled but disabled opacity, we at least check disabled
      // toggle OFF gris deshabilitado: sim toggle should be disabled/off (store check above suffices, plus UI if RouteToggle exists)
      // rutas grises: if RouteButtons exists, they should be gris disabled; store selectedRoute null ensures that

      // 7) fetch abortado: if a fetch was in vuelo, AbortController abort called
      // In this test we pressed disconnect without in-flight fetch, but abort path should still be exercised via spy count
      // We assert abort was either called or clearPending was called (both required). For via App disconnect, at least purga happened.
      // For stricter abort test see useConnection.disconnect.test

      clearInterval(fakeInterval);
      clearIntervalSpy.mockRestore();
      setIntervalSpy.mockRestore();
      jest.useRealTimers();
    });

    it('press Disconnect aborts fetch en vuelo (AbortController abort)', async () => {
      // Arrange
      jest.useFakeTimers();
      // Make fetch hang
      const fetchMock = jest.fn(() => new Promise(() => {}));
      (global as any).fetch = fetchMock;
      const { useConnection } = await import('../hooks/useConnection');
      const { renderHook } = await import('@testing-library/react-native');
      // Start connecting fetch
      const { result } = renderHook(() => useConnection());
      // Need valid plate and OK net/db
      useAppStore.setState({ conn: 'idle', net: 'OK', db: 'OK', plate: '' } as any);
      const connectPromise = result.current.connect({ plate: 'TGY589', lat: 0, lon: 0, speed: 0 } as any);
      await act(async () => {
        jest.advanceTimersByTime(0);
        await Promise.resolve();
      });
      expect(useAppStore.getState().conn).toBe('connecting');

      // Act - disconnect while fetch pending should abort
      await act(async () => {
        result.current.disconnect();
        await Promise.resolve();
      });

      // Assert
      expect(abortSpy).toHaveBeenCalled();
      expect(useAppStore.getState().conn).toBe('idle');
      expect(useAppStore.getState().plate).toBe('');
      // also purga
      expect(mockClearPending).toHaveBeenCalled();

      // cleanup timeout
      jest.useRealTimers();
      // silence unhandled promise
      (connectPromise as any).catch(() => {});
    });

    it('Connect vuelve verde disabled tras Disconnect (validación plate)', async () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', plate: 'TGY589', sync: 'CONNECTED' } as any);
      const { getByTestId } = render(<App />);

      // Act
      await act(async () => {
        fireEvent.press(getByTestId('disconnect-btn'));
        await Promise.resolve();
      });

      // Assert
      const connectBtn = getByTestId('connect-btn');
      (expect(connectBtn) as any).toBeDisabled();
      // input "" check: PlateInput should have value ""
      const input = getByTestId('plate-input');
      expect(input.props.value).toBe('');
    });

    it('Activar ruta simulada vuelve OFF gris deshabilitado y rutas grises tras Disconnect', async () => {
      // Arrange
      useAppStore.setState({
        conn: 'connected',
        plate: 'TGY589',
        sync: 'CONNECTED',
        simOn: true,
        simEnabled: true,
        selectedRoute: 'bogota' as any,
      } as any);
      const { getByTestId, queryByTestId, getByText, queryByText } = render(<App />);

      // Act
      await act(async () => {
        fireEvent.press(getByTestId('disconnect-btn'));
        await Promise.resolve();
      });

      // Assert
      expect(useAppStore.getState().simOn).toBe(false);
      expect(useAppStore.getState().simEnabled).toBe(false);
      // If RouteToggle/RouteButtons exist, they should be disabled gris #e5e7eb
      const toggle = queryByTestId('sim-toggle');
      if (toggle) {
        expect(toggle.props.accessibilityState?.disabled || toggle.props.disabled).toBeTruthy();
        const dump = JSON.stringify(toggle.props.style ?? '');
        // Off gris is #e5e7eb or disabled style
        expect(dump.includes('#e5e7eb') || toggle.props.disabled === true || toggle.props.accessibilityState?.disabled === true).toBeTruthy();
      }
      const medBtn = queryByTestId('route-medellin-btn') ?? queryByText(/Medell/i);
      const bogBtn = queryByTestId('route-bogota-btn') ?? queryByText(/Bogot/i);
      if (medBtn) {
        const dump = JSON.stringify((medBtn as any).props?.style ?? '');
        expect(dump.includes('#e5e7eb') || (medBtn as any).props?.disabled === true).toBeTruthy();
      }
      if (bogBtn) {
        const dump = JSON.stringify((bogBtn as any).props?.style ?? '');
        expect(dump.includes('#e5e7eb') || (bogBtn as any).props?.disabled === true).toBeTruthy();
      }
    });
  });
});
