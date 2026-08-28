export const Accuracy = { High: 6, Balanced: 3, Low: 1, Lowest: 0 };
export const requestForegroundPermissionsAsync = jest.fn(async (): Promise<{ status: string }> => ({ status: 'denied' }));
export const getCurrentPositionAsync = jest.fn(async (): Promise<{ coords: { latitude: number; longitude: number; speed: number | null } }> => ({ coords: { latitude: 6.2442, longitude: -75.5812, speed: 0 } }));
export const watchPositionAsync = jest.fn(async (_opts: unknown, _cb: unknown): Promise<{ remove: () => void }> => ({ remove: jest.fn() }));
