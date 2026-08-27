export const requestForegroundPermissionsAsync = jest.fn(async (): Promise<{ status: string }> => ({ status: 'denied' }));
export const getCurrentPositionAsync = jest.fn(async (): Promise<{ coords: { latitude: number; longitude: number; speed: number | null } }> => ({ coords: { latitude: 6.2442, longitude: -75.5812, speed: 0 } }));
