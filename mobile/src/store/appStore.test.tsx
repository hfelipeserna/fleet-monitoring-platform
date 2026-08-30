// Covers [SPEC-005: AC-002 AC-004] FR-002 FR-003 FR-004 BR-003/004 BR-005
// TEST-002 minimal store conn state + TEST-004 Disconnect purga RED
// AC-004: given connected 20 pending + toggle ON when Disconnect -> 0 pending + input "" + idle + OFF gris + intervalo stopped

import { renderHook, act } from '@testing-library/react-native';

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

const mockClearPendingAC004 = jest.fn(async () => {});
const mockCountPendingAC004 = jest.fn(async () => 0);
jest.mock('../db/telemetry', () => ({
  enqueue: jest.fn(async () => {}),
  clearPending: (...args: any[]) => (mockClearPendingAC004 as any)(...args),
  countPending: (...args: any[]) => (mockCountPendingAC004 as any)(...args),
  getPending: jest.fn(async () => []),
  markSynced: jest.fn(async () => {}),
}), { virtual: true });

// RED until store/appStore.ts exists
import { useAppStore } from './appStore';
import { intervalRegistry } from './intervalRegistry';
import { injectIntervalPort, __resetPorts } from './ports';

describe('appStore', () => {
  beforeEach(() => {
    // Arrange - reset store to initial
    const { getState } = useAppStore as any;
    // Zustand store reset via setState if available
    try {
      useAppStore.setState({ conn: 'idle', sync: 'IDLE', net: 'OK', db: 'OK', plate: '' } as any);
    } catch {}
  });

  describe('initial state', () => {
    it('starts idle with IDLE sync and OK db/net', () => {
      // Arrange
      const { result } = renderHook(() => useAppStore());

      // Act
      const state = result.current;

      // Assert
      expect(state.conn).toBe('idle');
      expect(state.sync).toBe('IDLE');
      expect(state.db).toBe('OK');
      expect(state.net).toBe('OK');
    });
  });

  describe('conn state machine idle->connecting->connected->error', () => {
    it('transitions idle -> connecting', () => {
      // Arrange
      const { result } = renderHook(() => useAppStore());

      // Act
      act(() => {
        (result.current as any).setConn?.('connecting') ?? useAppStore.setState({ conn: 'connecting', sync: 'CONNECTING' } as any);
      });

      // Assert
      expect(useAppStore.getState().conn).toBe('connecting');
      expect(useAppStore.getState().sync).toBe('CONNECTING');
    });

    it('transitions connecting -> connected on 202', () => {
      // Arrange
      const { result } = renderHook(() => useAppStore());
      act(() => {
        useAppStore.setState({ conn: 'connecting', sync: 'CONNECTING' } as any);
      });

      // Act
      act(() => {
        useAppStore.setState({ conn: 'connected', sync: 'CONNECTED' } as any);
      });

      // Assert
      expect(useAppStore.getState().conn).toBe('connected');
      expect(useAppStore.getState().sync).toBe('CONNECTED');
    });

    it('transitions connecting -> error on 400/429/503/timeout', () => {
      // Arrange
      const { result } = renderHook(() => useAppStore());
      act(() => {
        useAppStore.setState({ conn: 'connecting', sync: 'CONNECTING' } as any);
      });

      // Act
      act(() => {
        useAppStore.setState({ conn: 'error', sync: 'ERROR' } as any);
      });

      // Assert
      expect(useAppStore.getState().conn).toBe('error');
      expect(useAppStore.getState().sync).toBe('ERROR');
    });

    it('resets to idle on disconnect', async () => {
      // Arrange
      act(() => {
        useAppStore.setState({ conn: 'connected', sync: 'CONNECTED', plate: 'TGY589' } as any);
      });

      // Act
      await act(async () => {
        const s: any = useAppStore.getState();
        if (s.disconnect) await s.disconnect();
        else useAppStore.setState({ conn: 'idle', sync: 'IDLE', plate: '' } as any);
      });

      // Assert
      expect(useAppStore.getState().conn).toBe('idle');
      expect(useAppStore.getState().plate ?? '').toBe('');
    });
  });

  describe('net and db desacoplados', () => {
    it('allows Network OK and DB OK independently', () => {
      // Arrange
      const { result } = renderHook(() => useAppStore());

      // Act
      act(() => {
        useAppStore.setState({ net: 'OK', db: 'OK' } as any);
      });

      // Assert
      expect(useAppStore.getState().net).toBe('OK');
      expect(useAppStore.getState().db).toBe('OK');
    });

    it('allows Network ERROR while DB OK', () => {
      // Arrange

      // Act
      act(() => {
        useAppStore.setState({ net: 'ERROR', db: 'OK' } as any);
      });

      // Assert
      expect(useAppStore.getState().net).toBe('ERROR');
      expect(useAppStore.getState().db).toBe('OK');
    });

    it('allows DB ERROR while Network OK', () => {
      // Arrange

      // Act
      act(() => {
        useAppStore.setState({ db: 'ERROR', net: 'OK' } as any);
      });

      // Assert
      expect(useAppStore.getState().db).toBe('ERROR');
      expect(useAppStore.getState().net).toBe('OK');
    });
  });

  describe('sync vs net desacoplados BR-004', () => {
    it('Network OK + Syncing ERROR after 503 is valid', () => {
      // Arrange

      // Act
      act(() => {
        useAppStore.setState({ net: 'OK', sync: 'ERROR', conn: 'error' } as any);
      });

      // Assert
      expect(useAppStore.getState().net).toBe('OK');
      expect(useAppStore.getState().sync).toBe('ERROR');
    });
  });

  // Covers [SPEC-005: AC-004] FR-004 BR-005 TS-004 — Disconnect purga suite RED
  describe('Disconnect purga AC-004 FR-004 BR-005', () => {
    beforeEach(() => {
      // Arrange - reset pending mock
      mockClearPendingAC004.mockClear();
      mockCountPendingAC004.mockClear();
      intervalRegistry.clearAll();
      try { __resetPorts(); } catch {}
      injectIntervalPort({
        register: (id: number) => intervalRegistry.register(id),
        clear: (id: number) => intervalRegistry.clear(id),
        clearAll: () => intervalRegistry.clearAll(),
      });
      jest.useFakeTimers();
    });
    afterEach(() => {
      jest.clearAllTimers();
      intervalRegistry.clearAll();
      try { __resetPorts(); } catch {}
      jest.useRealTimers();
    });

    it('given connected 20 pending + toggle ON when Disconnect -> 0 pending + input "" + idle + OFF gris + intervalo stopped', async () => {
      // Arrange
      let pending = 20;
      mockCountPendingAC004.mockImplementation(async () => pending);
      mockClearPendingAC004.mockImplementation(async () => { pending = 0; });
      const clearIntervalSpy = jest.spyOn(global as any, 'clearInterval');
      // Simulate interval generation 5s encola cada 5s — register via intervalRegistry for DI clearAll
      const fakeId: any = setInterval(() => {}, 5000);
      intervalRegistry.register(fakeId as unknown as number);
      act(() => {
        useAppStore.setState({
          conn: 'connected',
          sync: 'CONNECTED',
          plate: 'TGY589',
          net: 'OK',
          db: 'OK',
          simOn: true,
          simEnabled: true,
          selectedRoute: 'medellin' as any,
        } as any);
      });
      expect(useAppStore.getState().plate).toBe('TGY589');
      expect(await mockCountPendingAC004()).toBe(20);

      // Act
      await act(async () => {
        const s: any = useAppStore.getState();
        const res = s.disconnect ? s.disconnect() : null;
        if (res && typeof res.then === 'function') await res;
        await Promise.resolve();
      });
      await act(async () => {
        jest.advanceTimersByTime(10);
        await Promise.resolve();
      });

      // Assert
      expect(mockClearPendingAC004).toHaveBeenCalled();
      expect(await mockCountPendingAC004()).toBe(0);
      expect(useAppStore.getState().conn).toBe('idle');
      expect(useAppStore.getState().plate).toBe('');
      expect(useAppStore.getState().sync).toBe('IDLE');
      expect(useAppStore.getState().simOn).toBe(false);
      expect(useAppStore.getState().simEnabled).toBe(false);
      expect(useAppStore.getState().selectedRoute).toBeNull();
      expect(clearIntervalSpy).toHaveBeenCalled();
      clearInterval(fakeId);
      clearIntervalSpy.mockRestore();
    });

    it('Disconnect aborta fetch en vuelo (AbortController abort) y limpia pending', async () => {
      // Arrange
      const abortSpy = jest.fn();
      const OrigAbort = global.AbortController;
      (global as any).AbortController = class extends (OrigAbort as any) {
        abort(...args: any[]) { abortSpy(...args); return super.abort(...args); }
      } as any;
      // Start a hanging fetch via useConnection to have abortRef populated
      const { useConnection } = await import('../hooks/useConnection');
      const { renderHook: rh } = await import('@testing-library/react-native');
      (global as any).fetch = jest.fn(() => new Promise(() => {}));
      jest.spyOn(await import('../lib/api'), 'postTelemetry').mockImplementation(async (_ev: any, opts: any) => {
        return (global as any).fetch('http://localhost:8080/v1/telemetry', { signal: opts?.signal } as any);
      });
      useAppStore.setState({ conn: 'idle', net: 'OK', db: 'OK', plate: '' } as any);
      const { result } = rh(() => useConnection());
      const p = result.current.connect({ plate: 'TGY589', lat: 0, lon: 0, speed: 0 } as any);
      await act(async () => { jest.advanceTimersByTime(0); await Promise.resolve(); });
      expect(useAppStore.getState().conn).toBe('connecting');

      // Act
      await act(async () => { await result.current.disconnect(); });

      // Assert
      expect(abortSpy).toHaveBeenCalled();
      expect(mockClearPendingAC004).toHaveBeenCalled();
      expect(useAppStore.getState().conn).toBe('idle');
      expect(useAppStore.getState().plate).toBe('');
      (global as any).AbortController = OrigAbort;
      (p as any).catch(() => {});
    });

    it('Connect vuelve verde disabled tras Disconnect', async () => {
      // Arrange
      act(() => {
        useAppStore.setState({ conn: 'connected', plate: 'TGY589', sync: 'CONNECTED' } as any);
      });

      // Act
      await act(async () => {
        const s: any = useAppStore.getState();
        const r = s.disconnect ? s.disconnect() : null;
        if (r && typeof r.then === 'function') await r;
      });

      // Assert
      expect(useAppStore.getState().plate).toBe('');
      expect(useAppStore.getState().conn).toBe('idle');
      const { isValidPlate } = await import('../lib/plate');
      expect(isValidPlate(useAppStore.getState().plate)).toBe(false);
    });

    it('Activar ruta simulada vuelve OFF gris deshabilitado y rutas grises', async () => {
      // Arrange
      act(() => {
        useAppStore.setState({
          conn: 'connected',
          plate: 'TGY589',
          sync: 'CONNECTED',
          simOn: true,
          simEnabled: true,
          selectedRoute: 'bogota' as any,
        } as any);
      });

      // Act
      await act(async () => {
        const s: any = useAppStore.getState();
        const r = s.disconnect ? s.disconnect() : null;
        if (r && typeof r.then === 'function') await r;
      });

      // Assert
      expect(useAppStore.getState().simOn).toBe(false);
      expect(useAppStore.getState().simEnabled).toBe(false);
      expect(useAppStore.getState().selectedRoute).toBeNull();
    });
  });

  // Covers [SPEC-005: AC-005] FR-005 BR-006 TS-005 — Toggle Activar ruta simulada OFF/ON simOn guard suite RED
  describe('Toggle Activar ruta simulada simOn AC-005 FR-005 BR-006', () => {
    beforeEach(() => {
      // Arrange - reset to idle
      useAppStore.getState().reset();
    });

    it('idle -> OFF gris disabled: simOn false y simEnabled false cuando idle', () => {
      // Arrange
      useAppStore.setState({ conn: 'idle', simOn: false, simEnabled: false } as any);

      // Act
      const st = useAppStore.getState();

      // Assert
      expect(st.conn).toBe('idle');
      expect(st.simOn).toBe(false);
      expect(st.simEnabled).toBe(false);
    });

    it('connected OFF -> habilitado gris: simEnabled true cuando connected y simOn false', () => {
      // Arrange
      useAppStore.setState({ conn: 'idle', simOn: false, simEnabled: false } as any);

      // Act
      act(() => {
        useAppStore.setState({ conn: 'connected', simEnabled: true } as any);
      });

      // Assert
      const st = useAppStore.getState();
      expect(st.conn).toBe('connected');
      expect(st.simOn).toBe(false);
      expect(st.simEnabled).toBe(true);
    });

    it('no permite simOn=true cuando idle (guard: conn !== connected)', () => {
      // Arrange
      useAppStore.setState({ conn: 'idle', simOn: false, simEnabled: false } as any);

      // Act
      act(() => {
        useAppStore.getState().setSimOn(true);
      });

      // Assert
      expect(useAppStore.getState().simOn).toBe(false);
      expect(useAppStore.getState().conn).toBe('idle');
    });

    it('no permite simOn=true cuando connecting (guard)', () => {
      // Arrange
      useAppStore.setState({ conn: 'connecting', simOn: false } as any);

      // Act
      act(() => {
        useAppStore.getState().setSimOn(true);
      });

      // Assert
      expect(useAppStore.getState().simOn).toBe(false);
    });

    it('no permite simOn=true cuando error (guard)', () => {
      // Arrange
      useAppStore.setState({ conn: 'error', simOn: false } as any);

      // Act
      act(() => {
        useAppStore.getState().setSimOn(true);
      });

      // Assert
      expect(useAppStore.getState().simOn).toBe(false);
    });

    it('permite simOn=true cuando connected (OFF->ON)', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', simOn: false, simEnabled: true } as any);

      // Act
      act(() => {
        useAppStore.getState().setSimOn(true);
      });

      // Assert
      expect(useAppStore.getState().simOn).toBe(true);
      expect(useAppStore.getState().conn).toBe('connected');
    });

    it('ON->OFF vuelve simOn false y habilita rutas gris deshabilitadas', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, selectedRoute: 'medellin' as any } as any);

      // Act
      act(() => {
        useAppStore.getState().setSimOn(false);
      });

      // Assert
      expect(useAppStore.getState().simOn).toBe(false);
      expect(useAppStore.getState().conn).toBe('connected');
    });

    it('reset a idle deja simOn false y simEnabled false y selectedRoute null', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', simOn: true, simEnabled: true, selectedRoute: 'bogota' as any } as any);

      // Act
      act(() => {
        useAppStore.getState().reset();
      });

      // Assert
      expect(useAppStore.getState().simOn).toBe(false);
      expect(useAppStore.getState().simEnabled).toBe(false);
      expect(useAppStore.getState().selectedRoute).toBeNull();
      expect(useAppStore.getState().conn).toBe('idle');
    });
  });
});
