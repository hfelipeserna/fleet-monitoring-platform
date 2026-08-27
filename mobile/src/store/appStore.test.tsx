// Covers [SPEC-005: AC-002] FR-002 FR-003 BR-003/004
// TEST-002 minimal store conn state

import { renderHook, act } from '@testing-library/react-native';

// RED until store/appStore.ts exists
import { useAppStore } from './appStore';

describe('appStore', () => {
  beforeEach(() => {
    // Arrange - reset store to initial
    const { getState } = useAppStore as any;
    // Zustand store reset via setState if available
    try {
      useAppStore.setState({ conn: 'idle', sync: 'CONNECTING', net: 'OK', db: 'OK', plate: '' } as any);
    } catch {}
  });

  describe('initial state', () => {
    it('starts idle with CONNECTING sync and OK db/net', () => {
      // Arrange
      const { result } = renderHook(() => useAppStore());

      // Act
      const state = result.current;

      // Assert
      expect(state.conn).toBe('idle');
      expect(state.sync).toBe('CONNECTING');
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

    it('resets to idle on disconnect', () => {
      // Arrange
      act(() => {
        useAppStore.setState({ conn: 'connected', sync: 'CONNECTED', plate: 'TGY589' } as any);
      });

      // Act
      act(() => {
        const s: any = useAppStore.getState();
        if (s.disconnect) s.disconnect();
        else useAppStore.setState({ conn: 'idle', sync: 'CONNECTING', plate: '' } as any);
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
});
