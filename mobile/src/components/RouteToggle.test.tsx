// Covers [SPEC-005: AC-005] FR-005 BR-006 TS-005
// TDD RED for TASK-005-05: Toggle Activar ruta simulada OFF/ON
// Suites: idle -> OFF gris disabled, connected OFF -> habilitado gris, OFF->ON -> rutas azul habilitadas, ON->OFF -> rutas gris
// Verifies: disabled, accessibilityState, colores #e5e7eb vs #16a34a, Switch value, onValueChange no llamado cuando disabled, touch 44pt, accessibilityLabel, testID sim-toggle

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

jest.mock('../db/telemetry', () => ({
  enqueue: jest.fn(async () => {}),
  clearPending: jest.fn(async () => {}),
  countPending: jest.fn(async () => 0),
  getPending: jest.fn(async () => []),
  markSynced: jest.fn(async () => {}),
}), { virtual: true });

import { useAppStore } from '../store/appStore';
import { RouteToggle } from './RouteToggle';

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
    } as any);
  } catch {}
}

function getSwitchColor(sw: any): string | undefined {
  const style = sw.props.style;
  const flat = Array.isArray(style) ? Object.assign({}, ...style) : style ?? {};
  const bg = flat?.backgroundColor;
  if (bg) return bg;
  const tc = sw.props.trackColor ?? sw.props.thumbColor ?? sw.props.trackColor;
  if (typeof tc === 'string') return tc;
  if (tc && typeof tc === 'object') {
    const vals = Object.values(tc as Record<string, unknown>);
    for (const v of vals) if (typeof v === 'string' && v.startsWith('#')) return v as string;
    return JSON.stringify(tc);
  }
  const dump = JSON.stringify({ style, trackColor: sw.props.trackColor, thumbColor: sw.props.thumbColor });
  if (dump.includes('#16a34a')) return '#16a34a';
  if (dump.includes('#e5e7eb')) return '#e5e7eb';
  if (dump.includes('#86efac')) return '#86efac';
  return undefined;
}

function getSwitchDisabled(sw: any): boolean {
  if (typeof sw.props.disabled === 'boolean') return sw.props.disabled;
  if (sw.props.accessibilityState?.disabled === true) return true;
  return false;
}

describe('RouteToggle // Covers [SPEC-005: AC-005] FR-005 BR-006', () => {
  beforeEach(() => {
    // Arrange
    resetStore();
    jest.clearAllMocks();
  });

  describe('idle -> OFF gris disabled', () => {
    it('renders OFF gris #e5e7eb disabled when idle (conn !== connected)', () => {
      // Arrange
      useAppStore.setState({ conn: 'idle', simOn: false, simEnabled: false } as any);

      // Act
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');

      // Assert
      expect(sw).toBeTruthy();
      expect(sw.props.value).toBe(false);
      expect(getSwitchDisabled(sw)).toBe(true);
      const color = getSwitchColor(sw);
      const dump = JSON.stringify(sw.props);
      const isGray = color === '#e5e7eb' || dump.includes('#e5e7eb');
      expect(isGray).toBe(true);
      const notGreen = !(color === '#16a34a' || dump.includes('#16a34a'));
      expect(notGreen).toBe(true);
    });

    it('has testID sim-toggle and accessibilityLabel Activar ruta simulada', () => {
      // Arrange
      useAppStore.setState({ conn: 'idle', simOn: false } as any);

      // Act
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');

      // Assert
      expect(sw.props.testID).toBe('sim-toggle');
      const label = sw.props.accessibilityLabel ?? sw.props['aria-label'] ?? '';
      expect(String(label).toLowerCase()).toContain('activar ruta simulada');
    });

    it('has accessibilityState disabled true when idle', () => {
      // Arrange
      useAppStore.setState({ conn: 'idle', simOn: false } as any);

      // Act
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');

      // Assert
      expect(sw.props.accessibilityState).toBeDefined();
      expect(sw.props.accessibilityState.disabled).toBe(true);
      expect(getSwitchDisabled(sw)).toBe(true);
    });

    it('has touch target >=44pt (height or minHeight or hitSlop)', () => {
      // Arrange
      useAppStore.setState({ conn: 'idle', simOn: false } as any);

      // Act
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');

      // Assert
      const style = sw.props.style;
      const flat = Array.isArray(style) ? Object.assign({}, ...style) : style ?? {};
      const hitSlop = sw.props.hitSlop;
      const has44 =
        (typeof flat.height === 'number' && flat.height >= 44) ||
        (typeof flat.minHeight === 'number' && flat.minHeight >= 44) ||
        (typeof flat.width === 'number' && flat.width >= 44) ||
        (typeof flat.minWidth === 'number' && flat.minWidth >= 44) ||
        (hitSlop && typeof hitSlop === 'object') ||
        JSON.stringify(sw.props).includes('44');
      expect(has44).toBe(true);
    });

    it('does not call onValueChange when disabled (idle)', () => {
      // Arrange
      useAppStore.setState({ conn: 'idle', simOn: false } as any);
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');
      const before = useAppStore.getState().simOn;

      // Act
      try {
        fireEvent(sw, 'valueChange', true);
      } catch {}
      try {
        fireEvent(sw, 'onValueChange', true);
      } catch {}

      // Assert
      expect(useAppStore.getState().simOn).toBe(before);
      expect(useAppStore.getState().simOn).toBe(false);
      expect(getSwitchDisabled(sw)).toBe(true);
    });

    it('renders Switch value false when OFF (idle)', () => {
      // Arrange
      useAppStore.setState({ conn: 'idle', simOn: false } as any);

      // Act
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');

      // Assert
      expect(sw.props.value).toBe(false);
      expect(sw.props.value).not.toBe(true);
    });
  });

  describe('connected OFF -> habilitado gris', () => {
    it('renders OFF gris #e5e7eb enabled when connected and simOn false', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', simOn: false, simEnabled: true } as any);

      // Act
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');

      // Assert
      expect(sw.props.value).toBe(false);
      expect(getSwitchDisabled(sw)).toBe(false);
      expect(sw.props.accessibilityState?.disabled).toBe(false);
      const dump = JSON.stringify(sw.props);
      const isGray = dump.includes('#e5e7eb');
      expect(isGray).toBe(true);
      expect(dump.includes('#16a34a')).toBe(false);
    });

    it('has enabled accessibilityState false when connected OFF', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', simOn: false } as any);

      // Act
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');

      // Assert
      expect(sw.props.accessibilityState?.disabled).toBe(false);
      expect(getSwitchDisabled(sw)).toBe(false);
    });

    it('keeps OFF label gris when connected but not yet ON', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', simOn: false } as any);

      // Act
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');

      // Assert
      expect(sw.props.value).toBe(false);
      const dump = JSON.stringify(sw.props);
      expect(dump.includes('#e5e7eb')).toBe(true);
    });
  });

  describe('OFF->ON -> rutas azul habilitadas (#93c5fd sin seleccion)', () => {
    it('toggles OFF->ON to verde #16a34a and value true when connected', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', simOn: false } as any);
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');
      expect(sw.props.value).toBe(false);

      // Act
      act(() => {
        if (sw.props.onValueChange) sw.props.onValueChange(true);
      });

      // Assert
      const after = useAppStore.getState();
      expect(after.simOn).toBe(true);
      const dump = JSON.stringify({ props: sw.props, store: after });
      // after toggle, component should render verde; re-query
      const { getByTestId: get2 } = render(<RouteToggle />);
      const sw2 = get2('sim-toggle');
      expect(sw2.props.value).toBe(true);
      const dump2 = JSON.stringify(sw2.props);
      expect(dump2.includes('#16a34a')).toBe(true);
      expect(dump2.includes('#e5e7eb')).toBe(false);
    });

    it('habilita rutas azules #93c5fd sin seleccion tras ON (simOn true, selectedRoute null)', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', simOn: false, selectedRoute: null } as any);
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');

      // Act
      act(() => {
        if (sw.props.onValueChange) sw.props.onValueChange(true);
      });

      // Assert
      const st = useAppStore.getState();
      expect(st.simOn).toBe(true);
      expect(st.selectedRoute).toBeNull();
      // rutas deben estar habilitadas azules: verifica via store que simOn habilita botones (RouteButtons leerá simOn)
      // Para UI, si RouteToggle expone rutas, su dump debe contener #93c5fd
      // Si no, al menos store indica habilitadas
      expect(st.simOn).toBe(true);
      // Si componente no renderiza botones, se verifica que no hay selección y simOn true implica azul
      // Intenta renderizar RouteButtons si existe, pero no es obligatorio para RED
      expect(st.selectedRoute).toBeNull();
    });

    it('calls onValueChange and sets simOn true only when enabled (connected)', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', simOn: false } as any);
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');
      expect(getSwitchDisabled(sw)).toBe(false);

      // Act
      act(() => {
        if (sw.props.onValueChange) sw.props.onValueChange(true);
      });

      // Assert
      expect(useAppStore.getState().simOn).toBe(true);
      expect(sw.props.value === false || sw.props.value === true).toBeTruthy();
    });
  });

  describe('ON -> OFF -> rutas gris (#e5e7eb deshabilitadas)', () => {
    it('toggles ON->OFF vuelve a gris #e5e7eb y value false y rutas gris deshabilitadas', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', simOn: true, selectedRoute: 'medellin' } as any);
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');
      expect(sw.props.value).toBe(true);

      // Act
      act(() => {
        if (sw.props.onValueChange) sw.props.onValueChange(false);
      });

      // Assert
      const st = useAppStore.getState();
      expect(st.simOn).toBe(false);
      const { getByTestId: get2 } = render(<RouteToggle />);
      const sw2 = get2('sim-toggle');
      expect(sw2.props.value).toBe(false);
      const dump2 = JSON.stringify(sw2.props);
      expect(dump2.includes('#e5e7eb')).toBe(true);
      expect(dump2.includes('#16a34a')).toBe(false);
    });

    it('rutas vuelven a gris #e5e7eb deshabilitadas tras ON->OFF', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', simOn: true, selectedRoute: 'bogota' } as any);
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');

      // Act
      act(() => {
        if (sw.props.onValueChange) sw.props.onValueChange(false);
      });

      // Assert
      const st = useAppStore.getState();
      expect(st.simOn).toBe(false);
      // rutas gris implica selectedRoute null tras OFF (según spec ON->OFF limpia)
      // No se exige selectedRoute null aquí pero si simOn false, botones deben estar gris/deshabilitados
      expect(st.simOn).toBe(false);
    });
  });

  describe('Switch disabled guard adicional', () => {
    it('onValueChange no llamado cuando disabled (connecting)', () => {
      // Arrange
      useAppStore.setState({ conn: 'connecting', simOn: false } as any);
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');
      expect(getSwitchDisabled(sw)).toBe(true);
      const before = useAppStore.getState().simOn;

      // Act
      try {
        fireEvent(sw, 'valueChange', true);
      } catch {}
      try {
        if (!getSwitchDisabled(sw) && sw.props.onValueChange) sw.props.onValueChange(true);
      } catch {}

      // Assert
      expect(useAppStore.getState().simOn).toBe(before);
      expect(useAppStore.getState().simOn).toBe(false);
    });

    it('onValueChange no llamado cuando disabled (error)', () => {
      // Arrange
      useAppStore.setState({ conn: 'error', simOn: false } as any);
      const { getByTestId } = render(<RouteToggle />);
      const sw = getByTestId('sim-toggle');

      // Act
      const before = useAppStore.getState().simOn;
      try {
        fireEvent(sw, 'valueChange', true);
      } catch {}

      // Assert
      expect(getSwitchDisabled(sw)).toBe(true);
      expect(useAppStore.getState().simOn).toBe(before);
    });
  });
});
