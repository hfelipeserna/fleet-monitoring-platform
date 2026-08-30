// Covers [SPEC-005: AC-006] FR-006 FR-007 BR-007 BR-008 TS-006
// TASK-005-06 TDD RED: Rutas Medellín/Bogotá con purga + verde seleccionado + encolado 5s + uuid + plate se mantiene

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
  _mockQueue: [],
}), { virtual: true });

// Mock routes ~20 pts each, speed variado 0/45/85, lat/lon reales Medellín/Bogotá
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
      isDisconnecting: false,
      __abortController: null,
      __telemetryInterval: null,
    } as any);
  } catch {}
}

function getBgColor(el: any): string | undefined {
  const style = el.props.style;
  const flat = Array.isArray(style) ? Object.assign({}, ...style) : style ?? {};
  return flat.backgroundColor as string | undefined;
}

jest.setTimeout(20000);
describe('RouteButtons // Covers [SPEC-005: AC-006] FR-006/007 BR-007/008 TS-006', () => {
  beforeEach(() => {
    // Arrange
    act(() => resetStore());
    jest.clearAllMocks();
    mockEnqueue.mockClear();
    mockClearPending.mockClear();
    intervalRegistry.reset();
    jest.useFakeTimers();
    (global as any).fetch = jest.fn().mockResolvedValue({
      status: 202,
      ok: true,
      headers: { get: jest.fn().mockReturnValue(null) } as any,
      json: async () => ({ accepted: true }),
    } as any);
  });

  afterEach(() => {
    intervalRegistry.reset();
    jest.clearAllTimers();
    jest.useRealTimers();
    jest.restoreAllMocks();
  });

  describe('connected ON -> click Medellin -> purga + Medellin verde Bogota azul + encolado 5s', () => {
    it('purga pending previo al seleccionar Medellín', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      mockClearPending.mockClear();
      const { getByTestId } = render(<App />);
      const medBtn = getByTestId('route-medellin-btn');
      expect(getBgColor(medBtn)).toBe('#93c5fd');

      // Act
      await act(async () => {
        fireEvent.press(medBtn);
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
      const medBtn = getByTestId('route-medellin-btn');
      const bogBtn = getByTestId('route-bogota-btn');

      // Act
      await act(async () => {
        fireEvent.press(medBtn);
        await Promise.resolve();
      });

      // Assert
      expect(getBgColor(medBtn)).toBe('#86efac');
      expect(getBgColor(bogBtn)).toBe('#93c5fd');
      expect(useAppStore.getState().plate).toBe('ACF356');
    });

    it('encola pending cada 5s con client_event_id uuid, lat/lon Medellín reales y speed 0/45/85 tras Medellín', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      mockEnqueue.mockClear();
      mockClearPending.mockClear();
      const { getByTestId } = render(<App />);
      const medBtn = getByTestId('route-medellin-btn');

      // Act
      await act(async () => {
        fireEvent.press(medBtn);
        await Promise.resolve();
      });
      act(() => {
        jest.advanceTimersByTime(5000);
      });
      await act(async () => {
        await Promise.resolve();
      });

      // Assert
      expect(mockEnqueue).toHaveBeenCalled();
      const firstCall = mockEnqueue.mock.calls[0][0];
      expect(firstCall.client_event_id).toMatch(UUID_RE);
      expect(firstCall.plate).toBe('TGY589');
      expect(firstCall.lat).toBeCloseTo(6.2442, 1);
      expect(firstCall.lon).toBeCloseTo(-75.5812, 1);
      expect([0, 45, 85]).toContain(firstCall.speed);
      // Verifica intervalRegistry registra 5s
      const ids = intervalRegistry.getIds();
      expect(ids.length).toBeGreaterThan(0);
      // And setInterval spy should have been called with 5000 (telemetry gen)
      // Check that at least 5s timer exists via advance: second tick
      mockEnqueue.mockClear();
      act(() => {
        jest.advanceTimersByTime(5000);
      });
      await act(async () => {
        await Promise.resolve();
      });
      expect(mockEnqueue).toHaveBeenCalled();
      const secondCall = mockEnqueue.mock.calls[0][0];
      expect(secondCall.speed).toBeDefined();
    });

    it('secuencia desde 0 tras seleccionar Medellín (primer punto Medellín, no continúa previo)', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: 'bogota' as any } as any));
      // Simulate previo había encolado bogotá, ahora purga y debe reiniciar en 0 de Medellín
      mockEnqueue.mockClear();
      mockClearPending.mockClear();
      const { getByTestId } = render(<App />);
      const medBtn = getByTestId('route-medellin-btn');

      // Act
      await act(async () => {
        fireEvent.press(medBtn);
        await Promise.resolve();
      });
      act(() => {
        jest.advanceTimersByTime(5000);
      });
      await act(async () => {
        await Promise.resolve();
      });

      // Assert
      expect(mockClearPending).toHaveBeenCalled();
      const call = mockEnqueue.mock.calls[0][0];
      expect(call.lat).toBeCloseTo(6.2442, 1);
      expect(call.lon).toBeCloseTo(-75.5812, 1);
      expect(useAppStore.getState().selectedRoute).toBe('medellin');
    });

    it('placa se mantiene al seleccionar Medellín', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'XYZ123', selectedRoute: null } as any));
      const { getByTestId } = render(<App />);

      // Act
      await act(async () => {
        fireEvent.press(getByTestId('route-medellin-btn'));
        await Promise.resolve();
      });

      // Assert
      expect(useAppStore.getState().plate).toBe('XYZ123');
      expect(useAppStore.getState().plate).not.toBe('');
    });
  });

  describe('click Bogota -> purga Medellin + Bogota verde reinicia 0', () => {
    it('purga Medellín y Bogotá pasa a verde #86efac, Medellín a azul #93c5fd', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: 'medellin' as any } as any));
      const { getByTestId } = render(<App />);
      const medBtn = getByTestId('route-medellin-btn');
      const bogBtn = getByTestId('route-bogota-btn');
      expect(getBgColor(medBtn)).toBe('#86efac');
      mockClearPending.mockClear();

      // Act
      await act(async () => {
        fireEvent.press(bogBtn);
        await Promise.resolve();
      });

      // Assert
      expect(mockClearPending).toHaveBeenCalled();
      expect(useAppStore.getState().selectedRoute).toBe('bogota');
      expect(getBgColor(bogBtn)).toBe('#86efac');
      expect(getBgColor(medBtn)).toBe('#93c5fd');
    });

    it('secuencia Bogotá reinicia 0: primer punto Bogotá 4.7110,-74.0721 tras switch', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: 'medellin' as any } as any));
      mockEnqueue.mockClear();
      mockClearPending.mockClear();
      const { getByTestId } = render(<App />);
      // First ensure medellin sequence running
      await act(async () => {
        fireEvent.press(getByTestId('route-medellin-btn'));
        await Promise.resolve();
      });
      act(() => {
        jest.advanceTimersByTime(5000);
      });
      await act(async () => {
        await Promise.resolve();
      });
      expect(mockEnqueue).toHaveBeenCalled();
      mockEnqueue.mockClear();
      mockClearPending.mockClear();

      // Act: switch to bogota -> should purge and next point is bogota[0]
      await act(async () => {
        fireEvent.press(getByTestId('route-bogota-btn'));
        await Promise.resolve();
      });
      act(() => {
        jest.advanceTimersByTime(5000);
      });
      await act(async () => {
        await Promise.resolve();
        await Promise.resolve();
      });

      // Assert
      expect(mockClearPending).toHaveBeenCalled();
      const call = mockEnqueue.mock.calls[0][0];
      expect(call.lat).toBeCloseTo(4.7110, 1);
      expect(call.lon).toBeCloseTo(-74.0721, 1);
      expect(call.plate).toBe('TGY589');
      expect(call.client_event_id).toMatch(UUID_RE);
    }, 10000);

    it('encola Bogota cada 5s con speed variado incluye 0 y 85', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      mockEnqueue.mockClear();
      const { getByTestId } = render(<App />);

      // Act
      await act(async () => {
        fireEvent.press(getByTestId('route-bogota-btn'));
        await Promise.resolve();
      });
      // Advance 3 intervalos 15s para obtener al menos 3 puntos con speeds diferentes
      act(() => {
        jest.advanceTimersByTime(5000);
      });
      await act(async () => {
        await Promise.resolve();
      });
      act(() => {
        jest.advanceTimersByTime(5000);
      });
      await act(async () => {
        await Promise.resolve();
      });
      act(() => {
        jest.advanceTimersByTime(5000);
      });
      await act(async () => {
        await Promise.resolve();
      });

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
      });

      // Assert
      expect(useAppStore.getState().plate).toBe('ACF356');
    });

    it('interval 5s real: setInterval con 5000 y pendiente con uuid cada tick', async () => {
      // Arrange
      const setSpy = jest.spyOn(global as any, 'setInterval');
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      const { getByTestId } = render(<App />);
      mockEnqueue.mockClear();

      // Act
      await act(async () => {
        fireEvent.press(getByTestId('route-medellin-btn'));
        await Promise.resolve();
      });

      // Assert interval 5s
      const calls5000 = setSpy.mock.calls.filter((c: any) => c[1] === 5000);
      expect(calls5000.length).toBeGreaterThan(0);
      // And fake timer advance produces enqueue
      act(() => {
        jest.advanceTimersByTime(5000);
      });
      await act(async () => {
        await Promise.resolve();
      });
      expect(mockEnqueue).toHaveBeenCalled();
      expect(mockEnqueue.mock.calls[0][0].client_event_id).toMatch(UUID_RE);
      setSpy.mockRestore();
    });
  });

  describe('colores spec y estados deshabilitados', () => {
    it('rutas gris #e5e7eb deshabilitadas cuando simOn OFF', () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: false, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      const { getByTestId } = render(<App />);
      const medBtn = getByTestId('route-medellin-btn');
      const bogBtn = getByTestId('route-bogota-btn');

      // Act
      // no press

      // Assert
      expect(getBgColor(medBtn)).toBe('#e5e7eb');
      expect(getBgColor(bogBtn)).toBe('#e5e7eb');
      expect(medBtn.props.accessibilityState?.disabled).toBe(true);
      expect(bogBtn.props.accessibilityState?.disabled).toBe(true);
    });

    it('rutas azul #93c5fd habilitadas sin selección cuando simOn ON', () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      const { getByTestId } = render(<App />);
      const medBtn = getByTestId('route-medellin-btn');
      const bogBtn = getByTestId('route-bogota-btn');

      // Act

      // Assert
      expect(getBgColor(medBtn)).toBe('#93c5fd');
      expect(getBgColor(bogBtn)).toBe('#93c5fd');
      expect(medBtn.props.accessibilityState?.disabled).toBe(false);
    });

    it('no encola si simOn OFF (toggle gris)', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: false, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      mockEnqueue.mockClear();
      render(<App />);

      // Act
      act(() => {
        jest.advanceTimersByTime(10000);
      });
      await act(async () => {
        await Promise.resolve();
      });

      // Assert
      expect(mockEnqueue).not.toHaveBeenCalled();
    });
  });

  describe('POST /batch cada 5s o >=50 y client_event_id sagrado', () => {
    it('encola con client_event_id uuid v4 único por evento', async () => {
      // Arrange
      act(() => useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, plate: 'TGY589', selectedRoute: null } as any));
      mockEnqueue.mockClear();
      const { getByTestId } = render(<App />);
      await act(async () => {
        fireEvent.press(getByTestId('route-medellin-btn'));
        await Promise.resolve();
      });
      act(() => {
        jest.advanceTimersByTime(5000);
      });
      await act(async () => {
        await Promise.resolve();
      });
      act(() => {
        jest.advanceTimersByTime(5000);
      });
      await act(async () => {
        await Promise.resolve();
      });

      // Act
      const ids = mockEnqueue.mock.calls.map((c: any) => c[0].client_event_id);

      // Assert
      expect(ids.length).toBeGreaterThanOrEqual(2);
      ids.forEach((id: string) => expect(id).toMatch(UUID_RE));
      expect(new Set(ids).size).toBe(ids.length);
    });
  });
});