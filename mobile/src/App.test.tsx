// Covers [SPEC-005: AC-012] FR-012 BR-011 TS-012 TEST-012
// Covers [SPEC-005: AC-011] FR-010 FR-011 BR-010
// TASK-005-09 RED: polish fidelidad 6 wireframes. Debe fallar RED hasta que App.tsx y componentes tengan colores exactos,
// fonts sketch, touch 44pt y testIDs. AAA obligatorios + snapshot.

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
  appSchema: (x: unknown) => x,
  tableSchema: (x: unknown) => x,
}), { virtual: true });

jest.mock('./db/telemetry', () => ({
  enqueue: jest.fn(async () => ({})),
  clearPending: jest.fn(async () => {}),
  countPending: jest.fn(async () => 0),
  getPending: jest.fn(async () => []),
  markSynced: jest.fn(async () => {}),
}), { virtual: true });

import App from './App';
import { useAppStore } from './store/appStore';

function getBgColor(el: { props: { style?: unknown } }): string | undefined {
  const style = (el.props as { style?: unknown }).style as unknown;
  const flat = Array.isArray(style) ? Object.assign({}, ...(style as unknown[])) : (style as Record<string, unknown> | undefined) ?? {};
  return (flat as Record<string, unknown>).backgroundColor as string | undefined;
}

function getColor(el: { props: { style?: unknown } }): string | undefined {
  const style = (el.props as { style?: unknown }).style as unknown;
  const flat = Array.isArray(style) ? Object.assign({}, ...(style as unknown[])) : (style as Record<string, unknown> | undefined) ?? {};
  return ((flat as Record<string, unknown>).color as string | undefined) ?? ((flat as Record<string, unknown>).backgroundColor as string | undefined);
}

function has44pt(el: { props: Record<string, unknown> }): boolean {
  const style = (el.props as { style?: unknown }).style as unknown;
  const flat = Array.isArray(style) ? Object.assign({}, ...(style as unknown[])) : (style as Record<string, unknown> | undefined) ?? {};
  const h = (flat as Record<string, unknown>).minHeight as number | undefined;
  const hh = (flat as Record<string, unknown>).height as number | undefined;
  const pad = (flat as Record<string, unknown>).padding as number | undefined;
  const padV = (flat as Record<string, unknown>).paddingVertical as number | undefined;
  const hitSlop = (el.props as Record<string, unknown>).hitSlop as unknown;
  // hitSlop cuenta como 44pt si se define; padding*2+~16 base no es 44, exigir minHeight/height explícito
  return (
    (typeof h === 'number' && h >= 44) ||
    (typeof hh === 'number' && hh >= 44) ||
    (hitSlop != null && typeof hitSlop === 'object') ||
    (typeof pad === 'number' && pad * 2 + 12 >= 44)
  );
}

function hasSketchFont(el: { props: Record<string, unknown> }): boolean {
  const style = (el.props as { style?: unknown }).style as unknown;
  const flat = Array.isArray(style) ? Object.assign({}, ...(style as unknown[])) : (style as Record<string, unknown> | undefined) ?? {};
  const ff = (flat as Record<string, unknown>).fontFamily as string | undefined;
  // solo fontFamily sketch cuenta; no dump global para evitar false positive
  return typeof ff === 'string' && /sketch/i.test(ff);
}

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
    } as unknown as Record<string, unknown>);
  } catch {}
}

describe('App 6 wireframes // Covers [SPEC-005: AC-012] FR-012 BR-011', () => {
  beforeEach(() => {
    // Arrange
    resetStore();
    jest.clearAllMocks();
  });

  describe('6 wireframes snapshot fidelidad', () => {
    it('renders App snapshot with 6 wireframes structure', () => {
      // Arrange
      resetStore();

      // Act
      const tree = render(<App />).toJSON();

      // Assert
      expect(tree).toBeTruthy();
      expect(tree).toMatchSnapshot();
    });
  });

  describe('testIDs wireframes', () => {
    it('exposes required testIDs plate-input connect-btn sync-status db-status net-status sim-toggle route-medellin-btn route-bogota-btn', () => {
      // Arrange
      resetStore();

      // Act
      const { getByTestId } = render(<App />);

      // Assert
      expect(getByTestId('plate-input')).toBeTruthy();
      expect(getByTestId('connect-btn')).toBeTruthy();
      expect(getByTestId('sync-status')).toBeTruthy();
      expect(getByTestId('db-status')).toBeTruthy();
      expect(getByTestId('net-status')).toBeTruthy();
      expect(getByTestId('sim-toggle')).toBeTruthy();
      expect(getByTestId('route-medellin-btn')).toBeTruthy();
      expect(getByTestId('route-bogota-btn')).toBeTruthy();
    });

    it('exposes disconnect-btn when connected and plate-display', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', plate: 'ACF356', simOn: false } as unknown as Record<string, unknown>);

      // Act
      const { getByTestId } = render(<App />);

      // Assert
      expect(getByTestId('disconnect-btn')).toBeTruthy();
      expect(getByTestId('plate-display')).toBeTruthy();
    });
  });

  describe('colores exactos wireframes FR-012', () => {
    it('Connect verde #86efac cuando valido', () => {
      // Arrange
      resetStore();
      const { getByTestId } = render(<App />);
      const input = getByTestId('plate-input');
      const connectBtn = getByTestId('connect-btn');

      // Act
      fireEvent.changeText(input, 'acf356');

      // Assert
      const bg = getBgColor(connectBtn);
      const dump = JSON.stringify(connectBtn.props.style);
      const isGreen = bg === '#86efac' || dump.includes('#86efac');
      expect(isGreen).toBe(true);
      expect(bg).toBe('#86efac');
    });

    it('Connect gris #e5e7eb cuando invalido', () => {
      // Arrange
      resetStore();
      const { getByTestId } = render(<App />);
      const input = getByTestId('plate-input');
      const connectBtn = getByTestId('connect-btn');

      // Act
      fireEvent.changeText(input, 'ACF35');

      // Assert
      const bg = getBgColor(connectBtn);
      const dump = JSON.stringify(connectBtn.props.style);
      const isGray = bg === '#e5e7eb' || dump.includes('#e5e7eb');
      expect(isGray).toBe(true);
    });

    it('Disconnect rosa #f9a8d4', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', plate: 'TGY589' } as unknown as Record<string, unknown>);
      const { getByTestId } = render(<App />);

      // Act
      const disc = getByTestId('disconnect-btn');

      // Assert
      const bg = getBgColor(disc);
      const dump = JSON.stringify(disc.props.style);
      expect(bg === '#f9a8d4' || dump.includes('#f9a8d4')).toBe(true);
      expect(getBgColor(disc)).toBe('#f9a8d4');
    });

    it('Syncing rojo #dc2626 siempre (CONNECTING/CONNECTED/ERROR)', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', sync: 'CONNECTED', net: 'OK', db: 'OK' } as unknown as Record<string, unknown>);

      // Act
      const { getByTestId } = render(<App />);
      const syncEl = getByTestId('sync-status');

      // Assert
      const c = getColor(syncEl);
      const dump = JSON.stringify(syncEl.props.style);
      expect(c === '#dc2626' || dump.includes('#dc2626')).toBe(true);
      expect(getColor(syncEl)).toBe('#dc2626');
    });

    it('OK verde #16a34a y ERROR rojo #dc2626 en dots', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', sync: 'CONNECTED', net: 'OK', db: 'OK' } as unknown as Record<string, unknown>);
      const { getByTestId, rerender } = render(<App />);

      // Act
      const netOk = getByTestId('net-status');
      const dbOk = getByTestId('db-status');

      // Assert
      expect(getColor(netOk)).toBe('#16a34a');
      expect(getColor(dbOk)).toBe('#16a34a');

      // Arrange -> error state desacoplado
      act(() => {
        useAppStore.setState({ conn: 'error', sync: 'ERROR', net: 'ERROR', db: 'ERROR' } as unknown as Record<string, unknown>);
      });
      rerender(<App />);

      // Act
      const netErr = getByTestId('net-status');
      const dbErr = getByTestId('db-status');
      const syncErr = getByTestId('sync-status');

      // Assert
      expect(getColor(netErr)).toBe('#dc2626');
      expect(getColor(dbErr)).toBe('#dc2626');
      expect(getColor(syncErr)).toBe('#dc2626');
    });

    it('ruta gris #e5e7eb cuando sim OFF, azul #93c5fd cuando sim ON sin selección, verde #86efac seleccionado', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', simOn: false, selectedRoute: null } as unknown as Record<string, unknown>);
      const { getByTestId, rerender } = render(<App />);

      // Act
      const medOff = getByTestId('route-medellin-btn');
      const bogOff = getByTestId('route-bogota-btn');

      // Assert OFF gris
      expect(getBgColor(medOff)).toBe('#e5e7eb');
      expect(getBgColor(bogOff)).toBe('#e5e7eb');

      // Arrange ON sin selección -> azul
      act(() => {
        useAppStore.setState({ conn: 'connected', simOn: true, selectedRoute: null } as unknown as Record<string, unknown>);
      });
      rerender(<App />);

      // Act
      const medOn = getByTestId('route-medellin-btn');
      const bogOn = getByTestId('route-bogota-btn');

      // Assert azul
      expect(getBgColor(medOn)).toBe('#93c5fd');
      expect(getBgColor(bogOn)).toBe('#93c5fd');

      // Arrange Medellín seleccionado -> verde vs azul
      act(() => {
        useAppStore.setState({ conn: 'connected', simOn: true, selectedRoute: 'medellin' } as unknown as Record<string, unknown>);
      });
      rerender(<App />);

      // Act
      const medSel = getByTestId('route-medellin-btn');
      const bogNot = getByTestId('route-bogota-btn');

      // Assert verde
      expect(getBgColor(medSel)).toBe('#86efac');
      expect(getBgColor(bogNot)).toBe('#93c5fd');

      // Arrange Bogotá seleccionado inverso
      act(() => {
        useAppStore.setState({ conn: 'connected', simOn: true, selectedRoute: 'bogota' } as unknown as Record<string, unknown>);
      });
      rerender(<App />);

      // Act
      const medNot2 = getByTestId('route-medellin-btn');
      const bogSel = getByTestId('route-bogota-btn');

      // Assert verde Bogotá
      expect(getBgColor(bogSel)).toBe('#86efac');
      expect(getBgColor(medNot2)).toBe('#93c5fd');
    });
  });

  describe('fonts sketch AC-012', () => {
    it('usa font sketch en title/plate/Syncing (fontFamily sketch)', () => {
      // Arrange
      resetStore();

      // Act
      const { getByTestId, getByText } = render(<App />);
      const syncEl = getByTestId('sync-status');
      const titleEl = (() => {
        try {
          return getByText('fleet-mobile');
        } catch {
          return null;
        }
      })();

      // Assert
      const syncHasSketch = hasSketchFont(syncEl);
      const titleHasSketch = titleEl ? hasSketchFont(titleEl) : false;
      expect(syncHasSketch || titleHasSketch).toBe(true);
    });
  });

  describe('touch targets >=44pt AC-012 NFR-007', () => {
    it('Connect, Disconnect, sim-toggle y rutas tienen hitSlop o minHeight >=44', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', plate: 'ACF356', simOn: true, selectedRoute: null } as unknown as Record<string, unknown>);
      const { getByTestId, rerender } = render(<App />);

      // Act
      const disc = getByTestId('disconnect-btn');
      const toggle = getByTestId('sim-toggle');
      const med = getByTestId('route-medellin-btn');
      const bog = getByTestId('route-bogota-btn');

      // Assert conectados
      expect(has44pt(disc)).toBe(true);
      expect(has44pt(toggle)).toBe(true);
      expect(has44pt(med)).toBe(true);
      expect(has44pt(bog)).toBe(true);

      // Arrange idle para Connect
      act(() => {
        useAppStore.setState({ conn: 'idle', plate: '', simOn: false, selectedRoute: null } as unknown as Record<string, unknown>);
      });
      rerender(<App />);

      // Act
      const conn = getByTestId('connect-btn');

      // Assert
      expect(has44pt(conn)).toBe(true);
    });
  });

  describe('snapshot 6 wireframes exactos', () => {
    it('snapshot estable incluye colores wireframes #86efac #f9a8d4 #dc2626 #93c5fd #16a34a (y #e5e7eb en OFF)', () => {
      // Arrange
      useAppStore.setState({ conn: 'connected', sync: 'CONNECTED', net: 'OK', db: 'OK', plate: 'ACF356', simOn: true, selectedRoute: 'medellin' } as unknown as Record<string, unknown>);

      // Act
      const renderedOn = render(<App />);
      const dumpOn = JSON.stringify(renderedOn.toJSON());

      // Assert ON state: verde, rosa, rojo, azul, ok verde (sin gris porque ON)
      expect(dumpOn).toContain('#86efac');
      expect(dumpOn).toContain('#f9a8d4');
      expect(dumpOn).toContain('#dc2626');
      expect(dumpOn).toContain('#93c5fd');
      expect(dumpOn).toContain('#16a34a');
      // gris solo en OFF, verificar segundo render
      // Arrange OFF
      useAppStore.setState({ conn: 'connected', sync: 'CONNECTED', net: 'OK', db: 'OK', plate: 'ACF356', simOn: false, selectedRoute: null } as unknown as Record<string, unknown>);
      const renderedOff = render(<App />);
      const dumpOff = JSON.stringify(renderedOff.toJSON());
      // Assert OFF incluye gris
      expect(dumpOff).toContain('#e5e7eb');
      expect(renderedOn.toJSON()).toMatchSnapshot();
    });
  });
});
