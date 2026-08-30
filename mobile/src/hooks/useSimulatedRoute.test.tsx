// Covers [SPEC-005: AC-007] FR-006 BR-007 TS-007
// TASK-005-08 TDD RED: ON->OFF purga buffer simulado, rutas gris deshabilitadas, GPS real cada 5s misma placa
// Suites: purga, selectedRoute null, gris #e5e7eb, placa intacta, Location mock cada 5s, WatermelonDB mock, fake timers

import React from 'react';
import { render, fireEvent, act } from '@testing-library/react-native';

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

const mockEnqueue = jest.fn(async (p: any) => ({ id: '1', client_event_id: p.client_event_id ?? '550e8400-e29b-41d4-a716-446655440000', ...p }));
const mockClearPending = jest.fn(async () => {});
const mockCountPending = jest.fn(async () => 0);
const mockGetPending = jest.fn(async () => []);
const mockMarkSynced = jest.fn(async () => {});

jest.mock('../db/telemetry', () => ({
  enqueue: (...args: any[]) => (mockEnqueue as any)(...args),
  clearPending: (...args: any[]) => (mockClearPending as any)(...args),
  countPending: (...args: any[]) => (mockCountPending as any)(...args),
  getPending: (...args: any[]) => (mockGetPending as any)(...args),
  markSynced: (...args: any[]) => (mockMarkSynced as any)(...args),
}), { virtual: true });

jest.mock('../routes/medellin', () => ({
  MEDELLIN_ROUTE: Array.from({ length: 20 }, (_, i) => ({
    lat: 6.2442 + i * 0.0003,
    lon: -75.5812 - i * 0.0007,
    speed: [45, 85, 0, 45, 85, 0, 45, 85, 0, 45, 85, 0, 45, 85, 0, 45, 85, 0, 45, 85][i],
  })),
}), { virtual: true });

jest.mock('../routes/bogota', () => ({
  BOGOTA_ROUTE: Array.from({ length: 20 }, (_, i) => ({
    lat: 4.7110 + i * 0.0003,
    lon: -74.0721 - i * 0.0007,
    speed: [85, 0, 45, 85, 0, 45, 85, 0, 45, 85, 0, 45, 85, 0, 45, 85, 0, 45, 85, 0][i],
  })),
}), { virtual: true });

import { useAppStore } from '../store/appStore';
import { intervalRegistry } from '../store/intervalRegistry';
import * as Location from 'expo-location';
import App from '../App';

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function resetStore() {
  try {
    useAppStore.setState({
      conn: 'idle',
      sync: 'CONNECTING',
      net: 'OK',
      db: 'OK',
      plate: '',
      simOn: false,
      simEnabled: false,
      selectedRoute: null,
      selectedRouteIdx: 0,
      isDisconnecting: false,
    } as any);
  } catch {}
}

function getBgColor(el: any): string | undefined {
  const style = el.props.style;
  const flat = Array.isArray(style) ? Object.assign({}, ...style) : style ?? {};
  return flat.backgroundColor as string | undefined;
}

jest.setTimeout(10000);
describe('useSimulatedRoute ON->OFF GPS real // Covers [SPEC-005: AC-007] FR-006 BR-007 TS-007', () => {
  beforeEach(() => {
    // Arrange
    act(() => resetStore());
    jest.clearAllMocks();
    mockEnqueue.mockClear();
    mockClearPending.mockClear();
    mockCountPending.mockClear();
    intervalRegistry.reset();
    jest.useFakeTimers();
    (global as any).fetch = jest.fn().mockResolvedValue({
      status: 202,
      ok: true,
      headers: { get: jest.fn().mockReturnValue(null) } as any,
      json: async () => ({ accepted: true }),
    } as any);
    // Mock expo-location granted + getCurrentPositionAsync
    (Location.requestForegroundPermissionsAsync as jest.Mock).mockResolvedValue({ status: 'granted' } as any);
    (Location.getCurrentPositionAsync as jest.Mock).mockResolvedValue({
      coords: { latitude: 6.2442, longitude: -75.5812, speed: 42 },
    } as any);
  });

  afterEach(() => {
    intervalRegistry.reset();
    jest.clearAllTimers();
    jest.useRealTimers();
    jest.restoreAllMocks();
    jest.clearAllMocks();
  });

  describe('given simulado Medellin verde, when ON->OFF -> purga + gris + Location mock called', () => {
    it('purga buffer simulado via clearPending llamado', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: 'medellin' as any, selectedRouteIdx: 5 } as any));
      mockClearPending.mockClear();
      const { getByTestId } = render(<App />);
      const toggle = getByTestId('sim-toggle');
      expect(toggle.props.value).toBe(true);

      // Act
      await act(async () => {
        fireEvent(toggle, 'valueChange', false);
        // toggleSimOn is async: wait microtask
        await Promise.resolve();
        await Promise.resolve();
        jest.advanceTimersByTime(0);
        await Promise.resolve();
      });

      // Assert
      expect(mockClearPending).toHaveBeenCalledTimes(1);
      expect(useAppStore.getState().simOn).toBe(false);
    });

    it('selectedRoute vuelve a null tras ON->OFF', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: 'medellin' as any } as any));
      const { getByTestId } = render(<App />);
      const toggle = getByTestId('sim-toggle');
      expect(useAppStore.getState().selectedRoute).toBe('medellin');

      // Act
      await act(async () => {
        fireEvent(toggle, 'valueChange', false);
        await Promise.resolve();
        await Promise.resolve();
      });

      // Assert
      expect(useAppStore.getState().selectedRoute).toBeNull();
      expect(useAppStore.getState().simOn).toBe(false);
    });

    it('rutas vuelven a gris #e5e7eb deshabilitadas tras ON->OFF', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: 'medellin' as any } as any));
      const { getByTestId } = render(<App />);
      const toggle = getByTestId('sim-toggle');
      const medBtn = getByTestId('route-medellin-btn');
      const bogBtn = getByTestId('route-bogota-btn');
      expect(getBgColor(medBtn)).toBe('#86efac');
      expect(getBgColor(bogBtn)).toBe('#93c5fd');

      // Act
      await act(async () => {
        fireEvent(toggle, 'valueChange', false);
        await Promise.resolve();
        await Promise.resolve();
      });

      // Assert
      expect(getBgColor(getByTestId('route-medellin-btn'))).toBe('#e5e7eb');
      expect(getBgColor(getByTestId('route-bogota-btn'))).toBe('#e5e7eb');
      expect(getByTestId('route-medellin-btn').props.accessibilityState?.disabled).toBe(true);
      expect(getByTestId('route-bogota-btn').props.accessibilityState?.disabled).toBe(true);
      expect(
        (getByTestId('route-medellin-btn').props as { disabled?: boolean }).disabled === true ||
          getByTestId('route-medellin-btn').props.accessibilityState?.disabled === true,
      ).toBe(true);
      expect(
        (getByTestId('route-bogota-btn').props as { disabled?: boolean }).disabled === true ||
          getByTestId('route-bogota-btn').props.accessibilityState?.disabled === true,
      ).toBe(true);
    });

    it('misma placa intacta tras ON->OFF', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'ACF356', selectedRoute: 'medellin' as any } as any));
      const { getByTestId } = render(<App />);
      const toggle = getByTestId('sim-toggle');

      // Act
      await act(async () => {
        fireEvent(toggle, 'valueChange', false);
        await Promise.resolve();
      });

      // Assert
      expect(useAppStore.getState().plate).toBe('ACF356');
      expect(useAppStore.getState().plate).not.toBe('');
    });

    it('empieza a transmitir GPS real expo-location cada 5s con Location.getCurrentPositionAsync llamado', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: 'medellin' as any } as any));
      mockEnqueue.mockClear();
      (Location.getCurrentPositionAsync as jest.Mock).mockClear();
      (Location.requestForegroundPermissionsAsync as jest.Mock).mockClear();
      (Location.requestForegroundPermissionsAsync as jest.Mock).mockResolvedValue({ status: 'granted' } as any);
      (Location.getCurrentPositionAsync as jest.Mock).mockResolvedValue({ coords: { latitude: 4.7110, longitude: -74.0721, speed: 77 } } as any);
      const { getByTestId } = render(<App />);
      const toggle = getByTestId('sim-toggle');

      // Act
      await act(async () => {
        fireEvent(toggle, 'valueChange', false);
        await Promise.resolve();
        await Promise.resolve();
      });
      // advance 5s
      await act(async () => {
        jest.advanceTimersByTime(5000);
        await Promise.resolve();
        await Promise.resolve();
        // flush pending promises from interval async
        await Promise.resolve();
      });
      // need to flush the async interval's promise queue: Location is async, so wait next tick
      await act(async () => {
        await Promise.resolve();
        await Promise.resolve();
        jest.advanceTimersByTime(0);
        await Promise.resolve();
      });

      // Assert
      expect(Location.requestForegroundPermissionsAsync).toHaveBeenCalled();
      expect(Location.getCurrentPositionAsync).toHaveBeenCalledTimes(1);
      expect(Location.getCurrentPositionAsync).toHaveBeenCalledWith(expect.anything());
    });

    it('Location cada 5s encola con misma placa y uuid y lat/lon reales', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: 'medellin' as any } as any));
      mockEnqueue.mockClear();
      (Location.requestForegroundPermissionsAsync as jest.Mock).mockResolvedValue({ status: 'granted' } as any);
      (Location.getCurrentPositionAsync as jest.Mock).mockResolvedValue({
        coords: { latitude: 6.1234, longitude: -75.4567, speed: 55 },
      } as any);
      const { getByTestId } = render(<App />);
      const toggle = getByTestId('sim-toggle');

      // Act
      await act(async () => {
        fireEvent(toggle, 'valueChange', false);
        await Promise.resolve();
      });
      await act(async () => {
        jest.advanceTimersByTime(5000);
        await Promise.resolve();
        await Promise.resolve();
      });
      await act(async () => {
        await Promise.resolve();
        jest.advanceTimersByTime(0);
        await Promise.resolve();
      });

      // Assert
      expect(mockEnqueue).toHaveBeenCalledTimes(1);
      const call = mockEnqueue.mock.calls[0][0];
      expect(call.plate).toBe('TGY589');
      expect(call.client_event_id).toMatch(UUID_RE);
      expect(call.lat).toBeCloseTo(6.1234, 2);
      expect(call.lon).toBeCloseTo(-75.4567, 2);
      expect(call.speed).toBe(55);
    });

    it('segundo tick 10s hace segundo Location y segundo enqueue misma placa', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: 'medellin' as any } as any));
      mockEnqueue.mockClear();
      (Location.requestForegroundPermissionsAsync as jest.Mock).mockResolvedValue({ status: 'granted' } as any);
      (Location.getCurrentPositionAsync as jest.Mock).mockResolvedValue({
        coords: { latitude: 6.1111, longitude: -75.2222, speed: 33 },
      } as any);
      const { getByTestId } = render(<App />);
      const toggle = getByTestId('sim-toggle');

      // Act
      await act(async () => {
        fireEvent(toggle, 'valueChange', false);
        await Promise.resolve();
      });
      await act(async () => {
        jest.advanceTimersByTime(5000);
        await Promise.resolve();
        await Promise.resolve();
        await Promise.resolve();
      });
      await act(async () => {
        jest.advanceTimersByTime(5000);
        await Promise.resolve();
        await Promise.resolve();
      });
      await act(async () => {
        await Promise.resolve();
        jest.advanceTimersByTime(0);
        await Promise.resolve();
      });

      // Assert
      expect(Location.getCurrentPositionAsync).toHaveBeenCalledTimes(2);
      expect(mockEnqueue).toHaveBeenCalledTimes(2);
      const plates = mockEnqueue.mock.calls.map((c: any) => c[0].plate);
      expect(plates.every((p: string) => p === 'TGY589')).toBe(true);
      const ids = mockEnqueue.mock.calls.map((c: any) => c[0].client_event_id);
      expect(new Set(ids).size).toBe(2);
      ids.forEach((id: string) => expect(id).toMatch(UUID_RE));
    });

    it('no llama LocationgetCurrentPositionAsync mientras ON (simulado) cada 5s es simulado no GPS', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: 'medellin' as any } as any));
      mockEnqueue.mockClear();
      (Location.getCurrentPositionAsync as jest.Mock).mockClear();
      (Location.requestForegroundPermissionsAsync as jest.Mock).mockClear();
      const { getByTestId: _ } = render(<App />);
      expect(useAppStore.getState().simOn).toBe(true);

      // Act
      await act(async () => {
        jest.advanceTimersByTime(5000);
        await Promise.resolve();
        await Promise.resolve();
      });
      await act(async () => {
        await Promise.resolve();
      });

      // Assert
      expect(Location.getCurrentPositionAsync).not.toHaveBeenCalled();
      expect(Location.requestForegroundPermissionsAsync).not.toHaveBeenCalled();
      expect(mockEnqueue).toHaveBeenCalledTimes(1);
      const call = mockEnqueue.mock.calls[0][0];
      expect(call.lat).toBeCloseTo(6.2442, 1);
    });
  });
});