import { useEffect } from 'react';
import NetInfo from '@react-native-community/netinfo';
import { useAppStore } from '../store/appStore';

function toNetStatus(state: { isConnected: boolean | null; isInternetReachable: boolean | null | undefined }): 'OK' | 'ERROR' {
  if (state.isConnected === true && state.isInternetReachable === true) return 'OK';
  return 'ERROR';
}

export function useNetInfo(): void {
  useEffect(() => {
    let mounted = true;

    NetInfo.fetch()
      .then((s) => {
        if (!mounted) return;
        const net = toNetStatus(s as { isConnected: boolean | null; isInternetReachable: boolean | null });
        useAppStore.setState({ net });
      })
      .catch(() => {
        if (!mounted) return;
        useAppStore.setState({ net: 'ERROR' });
      });

    const unsubscribe = NetInfo.addEventListener((s) => {
      if (!mounted) return;
      const net = toNetStatus(s as { isConnected: boolean | null; isInternetReachable: boolean | null });
      useAppStore.setState({ net });
    });

    return () => {
      mounted = false;
      try {
        unsubscribe();
      } catch {}
    };
  }, []);
}
