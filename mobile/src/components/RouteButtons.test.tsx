// Covers [SPEC-005: AC-006] FR-006 FR-007 BR-007 BR-008 TS-006

import React from 'react';
import { render, fireEvent, act, cleanup } from '@testing-library/react-native';

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

jest.mock('../db', () => ({ initDatabase: jest.fn(async () => 'OK') }));

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
  _mockQueue: [],
}), { virtual: true });

jest.mock('../hooks/useSync', () => ({
  useSync: jest.fn(),
}));

jest.mock('../hooks/useNetInfo', () => ({
  useNetInfo: jest.fn(),
}));

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
import { injectTelemetryPort, injectIntervalPort } from '../store/ports';
import { __resetPorts } from '../store/ports';
import App from '../App';

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function getBgColor(el: any): string | undefined {
  const style = el.props.style;
  const flat = Array.isArray(style) ? Object.assign({}, ...style) : style ?? {};
  return flat.backgroundColor as string | undefined;
}

async function flushMicrotasks() {
  // Arrange helper: drains microtasks inside act (no flakiness)
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

async function tick(ms: number) {
  await act(async () => {
    jest.advanceTimersByTime(ms);
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe('RouteButtons // Covers [SPEC-005: AC-006] FR-006/007 BR-007/008 TS-006', () => {
  beforeEach(() => {
    jest.restoreAllMocks();
    jest.useFakeTimers({ legacyFakeTimers: false });
    jest.clearAllTimers();
    __resetPorts();
    intervalRegistry.reset();
    act(() => useAppStore.getState().reset());
    injectTelemetryPort({
      clearPending: (...args: any[]) => (mockClearPending as any)(...args),
      enqueue: (...args: any[]) => (mockEnqueue as any)(...args),
      countPending: (...args: any[]) => (mockCountPending as any)(...args),
      getPending: (...args: any[]) => (mockGetPending as any)(...args),
      markSynced: (...args: any[]) => (mockMarkSynced as any)(...args),
    } as any);
    injectIntervalPort({
      register: (id: number) => intervalRegistry.register(id),
      clear: (id: number) => intervalRegistry.clear(id),
      clearAll: () => intervalRegistry.clearAll(),
    });
    mockEnqueue.mockClear();
    mockClearPending.mockClear();
    mockCountPending.mockClear();
    mockCountPending.mockResolvedValue(0);
    mockGetPending.mockClear();
    mockGetPending.mockResolvedValue([]);
    mockMarkSynced.mockClear();
    (global as any).fetch = jest.fn().mockResolvedValue({
      status: 202,
      ok: true,
      headers: { get: jest.fn().mockReturnValue(null) } as any,
      json: async () => ({ accepted: true }),
    } as any);
  });

  afterEach(() => {
    jest.clearAllTimers();
    cleanup();
    intervalRegistry.reset();
    __resetPorts();
    act(() => useAppStore.getState().reset());
    jest.useRealTimers();
    jest.restoreAllMocks();
  });

  describe('connected ON -> click Medellin -> purga + Medellin verde Bogota azul + encolado 5s // Covers [SPEC-005: AC-006]', () => {
    it('purga pending previo al seleccionar Medellín', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      const { getByTestId } = render(<App />);
      const medBtn = getByTestId('route-medellin-btn');
      expect(getBgColor(medBtn)).toBe('#93c5fd');

      // Act
      await act(async () => {
        fireEvent.press(medBtn);
        await Promise.resolve();
        await Promise.resolve();
      });

      // Assert
      expect(mockClearPending).toHaveBeenCalledTimes(1);
      expect(useAppStore.getState().selectedRoute).toBe('medellin');
      expect(useAppStore.getState().plate).toBe('TGY589');
    });

    it('Medellín verde #86efac y Bogotá azul #93c5fd tras click Medellín', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'ACF356', selectedRoute: null } as any));
      const { getByTestId } = render(<App />);

      // Act
      await act(async () => {
        fireEvent.press(getByTestId('route-medellin-btn'));
        await Promise.resolve();
        await Promise.resolve();
      });

      // Assert
      expect(getBgColor(getByTestId('route-medellin-btn'))).toBe('#86efac');
      expect(getBgColor(getByTestId('route-bogota-btn'))).toBe('#93c5fd');
      expect(useAppStore.getState().plate).toBe('ACF356');
    });

    it('encola pending cada 5s con client_event_id uuid, lat/lon Medellín reales y speed 0/45/85 tras Medellín', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      const { getByTestId } = render(<App />);
      const medBtn = getByTestId('route-medellin-btn');
      expect(getBgColor(medBtn)).toBe('#93c5fd');

      // Act
      await act(async () => {
        fireEvent.press(medBtn);
        await Promise.resolve();
        await Promise.resolve();
      });
      await tick(5000);
      await flushMicrotasks();
      // capture first enqueue after first tick
      expect(mockEnqueue).toHaveBeenCalledTimes(1);
      const firstCall = mockEnqueue.mock.calls[0][0];
      mockEnqueue.mockClear();
      await tick(5000);
      await flushMicrotasks();

      // Assert
      expect(mockClearPending).toHaveBeenCalledTimes(1);
      expect(useAppStore.getState().selectedRoute).toBe('medellin');
      expect(useAppStore.getState().plate).toBe('TGY589');
      expect(getBgColor(getByTestId('route-medellin-btn'))).toBe('#86efac');
      expect(intervalRegistry.getIds().length).toBe(2);
      expect(firstCall.client_event_id).toMatch(UUID_RE);
      expect(firstCall.plate).toBe('TGY589');
      expect(firstCall.lat).toBeCloseTo(6.2442, 1);
      expect(firstCall.lon).toBeCloseTo(-75.5812, 1);
      expect([0, 45, 85]).toContain(firstCall.speed);
      expect(mockEnqueue).toHaveBeenCalledTimes(1);
      const secondCall = mockEnqueue.mock.calls[0][0];
      expect(secondCall.client_event_id).toMatch(UUID_RE);
      expect(secondCall.client_event_id).not.toBe(firstCall.client_event_id);
      expect(secondCall.speed).toBeDefined();
    });

    it('secuencia desde 0 tras seleccionar Medellín (primer punto Medellín, no continúa previo)', async () => {
      // Arrange
      act(() =>
        useAppStore.setState({
          conn: 'connected',
          simOn: true,
          simEnabled: true,
          plate: 'TGY589',
          selectedRoute: 'bogota' as any,
          selectedRouteIdx: 7,
        } as any),
      );
      const { getByTestId } = render(<App />);
      const medBtn = getByTestId('route-medellin-btn');

      // Act
      await act(async () => {
        fireEvent.press(medBtn);
        await Promise.resolve();
        await Promise.resolve();
      });
      expect(useAppStore.getState().selectedRouteIdx).toBe(0);
      await tick(5000);
      await flushMicrotasks();

      // Assert
      expect(mockClearPending).toHaveBeenCalled();
      expect(useAppStore.getState().selectedRoute).toBe('medellin');
      const call = mockEnqueue.mock.calls[0][0];
      expect(call.lat).toBeCloseTo(6.2442, 1);
      expect(call.lon).toBeCloseTo(-75.5812, 1);
      expect(call.client_event_id).toMatch(UUID_RE);
      expect(useAppStore.getState().selectedRouteIdx).toBe(1);
    });

    it('placa se mantiene al seleccionar Medellín', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'XYZ123', selectedRoute: null } as any));
      const { getByTestId } = render(<App />);

      // Act
      await act(async () => {
        fireEvent.press(getByTestId('route-medellin-btn'));
        await Promise.resolve();
        await Promise.resolve();
      });

      // Assert
      expect(useAppStore.getState().plate).toBe('XYZ123');
      expect(useAppStore.getState().plate).not.toBe('');
    });
  });

  describe('click Bogota -> purga Medellin + Bogota verde reinicia 0 // Covers [SPEC-005: AC-006]', () => {
    it('purga Medellín y Bogotá pasa a verde #86efac, Medellín a azul #93c5fd', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: 'medellin' as any } as any));
      const { getByTestId } = render(<App />);
      expect(getBgColor(getByTestId('route-medellin-btn'))).toBe('#86efac');

      // Act
      await act(async () => {
        fireEvent.press(getByTestId('route-bogota-btn'));
        await Promise.resolve();
        await Promise.resolve();
      });

      // Assert
      expect(mockClearPending).toHaveBeenCalled();
      expect(useAppStore.getState().selectedRoute).toBe('bogota');
      expect(getBgColor(getByTestId('route-bogota-btn'))).toBe('#86efac');
      expect(getBgColor(getByTestId('route-medellin-btn'))).toBe('#93c5fd');
    });

    it('secuencia Bogotá reinicia 0: primer punto Bogotá 4.7110,-74.0721 tras switch', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: 'medellin' as any, selectedRouteIdx: 5 } as any));
      const { getByTestId } = render(<App />);

      // Act
      await act(async () => {
        fireEvent.press(getByTestId('route-bogota-btn'));
        await Promise.resolve();
        await Promise.resolve();
      });
      await tick(5000);
      await flushMicrotasks();

      // Assert
      expect(mockClearPending).toHaveBeenCalled();
      const call = mockEnqueue.mock.calls[0][0];
      expect(call.lat).toBeCloseTo(4.7110, 1);
      expect(call.lon).toBeCloseTo(-74.0721, 1);
      expect(call.plate).toBe('TGY589');
      expect(call.client_event_id).toMatch(UUID_RE);
    });

    it('encola Bogota cada 5s con speed variado incluye 0 y 85', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      const { getByTestId } = render(<App />);

      // Act
      await act(async () => {
        fireEvent.press(getByTestId('route-bogota-btn'));
        await Promise.resolve();
        await Promise.resolve();
      });
      await tick(5000);
      await tick(5000);
      await tick(5000);
      await flushMicrotasks();

      // Assert
      expect(mockEnqueue.mock.calls.length).toBeGreaterThanOrEqual(3);
      const speeds = mockEnqueue.mock.calls.map((c: any) => c[0].speed);
      expect(speeds.some((s: number) => s === 0)).toBe(true);
      expect(speeds.some((s: number) => s === 85)).toBe(true);
    });

    it('placa se mantiene al cambiar de Medellín a Bogotá', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'ACF356', selectedRoute: 'medellin' as any } as any));
      const { getByTestId } = render(<App />);

      // Act
      await act(async () => {
        fireEvent.press(getByTestId('route-bogota-btn'));
        await Promise.resolve();
        await Promise.resolve();
      });

      // Assert
      expect(useAppStore.getState().plate).toBe('ACF356');
    });

    it('interval 5s real: setInterval con 5000 y pendiente con uuid cada tick', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      const { getByTestId } = render(<App />);

      // Act
      await act(async () => {
        fireEvent.press(getByTestId('route-medellin-btn'));
        await Promise.resolve();
        await Promise.resolve();
      });
      await tick(5000);
      await flushMicrotasks();

      // Assert
      expect(intervalRegistry.getIds().length).toBe(2);
      expect(mockEnqueue).toHaveBeenCalled();
      expect(mockEnqueue.mock.calls[0][0].client_event_id).toMatch(UUID_RE);
    });
  });

  describe('colores spec y estados deshabilitados // Covers [SPEC-005: AC-006]', () => {
    it('rutas gris #e5e7eb deshabilitadas cuando simOn OFF', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: false, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      const { getByTestId } = render(<App />);
      const medBtn = getByTestId('route-medellin-btn');
      const bogBtn = getByTestId('route-bogota-btn');

      // Act
      await flushMicrotasks();

      // Assert
      expect(getBgColor(medBtn)).toBe('#e5e7eb');
      expect(getBgColor(bogBtn)).toBe('#e5e7eb');
      expect(medBtn.props.accessibilityState?.disabled).toBe(true);
      expect(bogBtn.props.accessibilityState?.disabled).toBe(true);
    });

    it('rutas azul #93c5fd habilitadas sin selección cuando simOn ON', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      const { getByTestId } = render(<App />);
      const medBtn = getByTestId('route-medellin-btn');
      const bogBtn = getByTestId('route-bogota-btn');

      // Act
      await flushMicrotasks();

      // Assert
      expect(getBgColor(medBtn)).toBe('#93c5fd');
      expect(getBgColor(bogBtn)).toBe('#93c5fd');
      expect(medBtn.props.accessibilityState?.disabled).toBe(false);
    });

    it('no encola si simOn OFF (toggle gris)', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: false, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      render(<App />);

      // Act
      await tick(10000);
      await flushMicrotasks();

      // Assert
      expect(mockEnqueue).not.toHaveBeenCalled();
    });
  });

  describe('POST /batch cada 5s o >=50 y client_event_id sagrado // Covers [SPEC-005: AC-006]', () => {
    it('encola con client_event_id uuid v4 único por evento', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      const { getByTestId } = render(<App />);
      await act(async () => {
        fireEvent.press(getByTestId('route-medellin-btn'));
        await Promise.resolve();
        await Promise.resolve();
      });

      // Act
      await tick(5000);
      await tick(5000);
      await flushMicrotasks();

      // Assert
      const ids = mockEnqueue.mock.calls.map((c: any) => c[0].client_event_id);
      expect(ids.length).toBeGreaterThanOrEqual(2);
      ids.forEach((id: string) => expect(id).toMatch(UUID_RE));
      expect(new Set(ids).size).toBe(ids.length);
    });
  });
});
