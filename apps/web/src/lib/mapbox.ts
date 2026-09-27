const TOKEN = process.env.NEXT_PUBLIC_MAPBOX_ACCESS_TOKEN ?? "";
const GEOCODE_ENDPOINT = "https://api.mapbox.com/geocoding/v5/mapbox.places";
const COUNTRY = "np";

export type GeocodeSuggestion = {
  id: string;
  placeName: string;
  center: [number, number];
};

type MapboxFeature = {
  id: string;
  place_name: string;
  center: [number, number];
};

export function isMapboxConfigured(): boolean {
  return TOKEN.length > 0;
}

export async function geocodeAddress(
  query: string,
  signal?: AbortSignal,
): Promise<GeocodeSuggestion[]> {
  const value = query.trim();
  if (!TOKEN || value.length < 3) {
    return [];
  }

  const url = new URL(`${GEOCODE_ENDPOINT}/${encodeURIComponent(value)}.json`);
  url.searchParams.set("access_token", TOKEN);
  url.searchParams.set("autocomplete", "true");
  url.searchParams.set("limit", "5");
  url.searchParams.set("country", COUNTRY);

  const response = await fetch(url.toString(), { signal });
  if (!response.ok) {
    return [];
  }

  const payload = (await response.json()) as { features?: MapboxFeature[] };
  return (payload.features ?? []).map((feature) => ({
    id: feature.id,
    placeName: feature.place_name,
    center: feature.center,
  }));
}

export function staticMapUrl(
  center: [number, number],
  options: { width?: number; height?: number; zoom?: number } = {},
): string {
  const [lng, lat] = center;
  const width = options.width ?? 600;
  const height = options.height ?? 200;
  const zoom = options.zoom ?? 14;
  const marker = `pin-s+3b82f6(${lng},${lat})`;
  return `https://api.mapbox.com/styles/v1/mapbox/streets-v12/static/${marker}/${lng},${lat},${zoom}/${width}x${height}@2x?access_token=${TOKEN}`;
}
