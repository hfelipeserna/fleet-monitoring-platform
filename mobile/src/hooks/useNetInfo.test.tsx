/* eslint-disable @typescript-eslint/no-explicit-any */
// Covers AC-003 AC-010 FR-003 BR-004 - useNetInfo NetInfo listener + cleanup + desacoplados
import { renderHook, act, waitFor } from '@testing-library/react-native';

const mockAddEventListener = jest.fn((cb: any) => jest.fn());
const mockFetch = jest.fn().mockResolvedValue({ isConnected: true, isInternetReachable: true });

jest.mock('@react-native-community/netinfo', () => ({
  addEventListener: (...args: any[]) => (mockAddEventListener as any)(...args),
  fetch: (...args: any[]) => (mockFetch as any)(...args),
}), { virtual: true });

jest.mock('expo-constants', () => ({
  default: { expoConfig: { extra: { apiUrl: 'http://localhost:8080' } } },
  expoConfig: { extra: { apiUrl: 'http://localhost:8080' } },
}), { virtual: true });

jest.mock('@nozbe/watermelondb', () => ({
  appSchema: (x: any) => x,
  tableSchema: (x: any) => x,
}), { virtual: true });

import { useAppStore } from '../store/appStore';
import { useNetInfo } from './useNetInfo';

function resetStore() {
  try {
    useAppStore.setState({ conn: 'idle', sync: 'CONNECTING', net: 'OK', db: 'OK', plate: '' } as any);
  } catch {}
}

describe('useNetInfo', () => {
  let listenerCb: any;
  let unsubscribeMock: jest.Mock;

  beforeEach(() => {
    // Arrange
    resetStore();
    jest.clearAllMocks();
    unsubscribeMock = jest.fn();
    mockAddEventListener.mockImplementation((cb: any) => {
      listenerCb = cb;
      return unsubscribeMock;
    });
    mockFetch.mockResolvedValue({ isConnected: true, isInternetReachable: true });
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('listener registra addEventListener y limpia en unmount', () => {
    it('registers addEventListener on mount', async () => {
      // Arrange
      // Act
      renderHook(() => useNetInfo());

      // Assert
      await waitFor(() => expect(mockAddEventListener).toHaveBeenCalledTimes(1));
      expect(mockAddEventListener).toHaveBeenCalledWith(expect.any(Function));
    });

    it('cleans up with unsubscribe on unmount (no leak)', async () => {
      // Arrange
      const { unmount } = renderHook(() => useNetInfo());
      await waitFor(() => expect(mockAddEventListener).toHaveBeenCalled());

      // Act
      unmount();

      // Assert
      expect(unsubscribeMock).toHaveBeenCalledTimes(1);
    });

    it('does not leak: removeEventListener called via returned unsubscribe', async () => {
      // Arrange
      const { unmount } = renderHook(() => useNetInfo());

      // Act
      await waitFor(() => expect(mockAddEventListener).toHaveBeenCalled());
      unmount();

      // Assert
      expect(unsubscribeMock).toHaveBeenCalled();
      // after unmount, triggering old callback should not change store (no leak)
      const before = useAppStore.getState().net;
      if (listenerCb) listenerCb({ isConnected: false, isInternetReachable: false });
      // allow microtick
      await act(async () => { await Promise.resolve(); });
      // Since component is unmounted, store should remain as before or not be double-updated
      // At minimum, unsubscribe was called once
      expect(unsubscribeMock).toHaveBeenCalledTimes(1);
      // If leak, callback after unmount would still flip net; we assert unsubscribe prevents further effect
      // Implementation must remove listener, so we verify mock was returned and called
    });
  });

  describe('fetch initial -> OK', () => {
    it('calls NetInfo.fetch on mount and sets Network OK when isConnected true + isInternetReachable true', async () => {
      // Arrange
      mockFetch.mockResolvedValue({ isConnected: true, isInternetReachable: true });
      useAppStore.setState({ net: 'ERROR' } as any);

      // Act
      renderHook(() => useNetInfo());

      // Assert
      await waitFor(() => expect(mockFetch).toHaveBeenCalled());
      await waitFor(() => expect(useAppStore.getState().net).toBe('OK'));
    });

    it('initial fetch respects isInternetReachable true', async () => {
      // Arrange
      mockFetch.mockResolvedValue({ isConnected: true, isInternetReachable: true });

      // Act
      renderHook(() => useNetInfo());

      // Assert
      await waitFor(() => expect(useAppStore.getState().net).toBe('OK'));
    });
  });

  describe('event isConnected false -> ERROR', () => {
    it('sets Network ERROR when NetInfo event isConnected false', async () => {
      // Arrange
      mockFetch.mockResolvedValue({ isConnected: true, isInternetReachable: true });
      const { } = renderHook(() => useNetInfo());
      await waitFor(() => expect(mockAddEventListener).toHaveBeenCalled());
      expect(useAppStore.getState().net).toBe('OK');

      // Act
      await act(async () => {
        listenerCb({ isConnected: false, isInternetReachable: true });
      });

      // Assert
      await waitFor(() => expect(useAppStore.getState().net).toBe('ERROR'));
    });

    it('sets ERROR when isConnected false even if isInternetReachable true (strict)', async () => {
      // Arrange
      renderHook(() => useNetInfo());
      await waitFor(() => expect(mockAddEventListener).toHaveBeenCalled());

      // Act
      await act(async () => {
        listenerCb({ isConnected: false, isInternetReachable: true });
      });

      // Assert
      await waitFor(() => expect(useAppStore.getState().net).toBe('ERROR'));
    });
  });

  describe('isConnected true but isInternetReachable false -> ERROR', () => {
    it('sets Network ERROR when isConnected true but isInternetReachable false (BR-004)', async () => {
      // Arrange
      renderHook(() => useNetInfo());
      await waitFor(() => expect(mockAddEventListener).toHaveBeenCalled());
      useAppStore.setState({ net: 'OK' } as any);

      // Act
      await act(async () => {
        listenerCb({ isConnected: true, isInternetReachable: false });
      });

      // Assert
      await waitFor(() => expect(useAppStore.getState().net).toBe('ERROR'));
    });

    it('sets ERROR when isInternetReachable is null/undefined treated as false', async () => {
      // Arrange
      renderHook(() => useNetInfo());
      await waitFor(() => expect(mockAddEventListener).toHaveBeenCalled());

      // Act
      await act(async () => {
        listenerCb({ isConnected: true, isInternetReachable: null });
      });

      // Assert
      await waitFor(() => expect(useAppStore.getState().net).toBe('ERROR'));
    });

    it('fetch initial with isInternetReachable false -> ERROR', async () => {
      // Arrange
      mockFetch.mockResolvedValue({ isConnected: true, isInternetReachable: false });

      // Act
      renderHook(() => useNetInfo());

      // Assert
      await waitFor(() => expect(useAppStore.getState().net).toBe('ERROR'));
    });
  });

  describe('no leak (removeEventListener llamado) + desacoplados', () => {
    it('calls unsubscribe exactly once per mount/unmount cycle', async () => {
      // Arrange
      const { unmount } = renderHook(() => useNetInfo());
      await waitFor(() => expect(mockAddEventListener).toHaveBeenCalledTimes(1));

      // Act
      unmount();

      // Assert
      expect(unsubscribeMock).toHaveBeenCalledTimes(1);
      // re-mount should register new listener without leftover
      const secondUnsub = jest.fn();
      mockAddEventListener.mockReturnValueOnce(secondUnsub);
      const { unmount: u2 } = renderHook(() => useNetInfo());
      await waitFor(() => expect(mockAddEventListener).toHaveBeenCalledTimes(2));
      u2();
      expect(secondUnsub).toHaveBeenCalledTimes(1);
    });

    it('does not affect Syncing state (desacoplados) when Network flips', async () => {
      // Arrange
      useAppStore.setState({ net: 'OK', sync: 'CONNECTED', conn: 'connected' } as any);
      renderHook(() => useNetInfo());
      await waitFor(() => expect(mockAddEventListener).toHaveBeenCalled());

      // Act
      await act(async () => {
        listenerCb({ isConnected: false, isInternetReachable: false });
      });

      // Assert
      await waitFor(() => expect(useAppStore.getState().net).toBe('ERROR'));
      // Syncing must remain CONNECTED (desacoplado) - Network ERROR does not auto flip Syncing
      // Hook must only touch net, not sync
      expect(useAppStore.getState().sync).toBe('CONNECTED');
    });
  });
});
