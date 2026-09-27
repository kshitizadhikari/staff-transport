"use client";

import { useEffect, useId, useState } from "react";

import { Input } from "@/components/ui/input";
import { useDebouncedValue } from "@/hooks/use-debounced-value";
import {
  geocodeAddress,
  isMapboxConfigured,
  staticMapUrl,
  type GeocodeSuggestion,
} from "@/lib/mapbox";

type AddressInputProps = {
  id?: string;
  value: string;
  onChange: (value: string) => void;
  onSelect?: (suggestion: GeocodeSuggestion) => void;
  placeholder?: string;
  disabled?: boolean;
  invalid?: boolean;
  showMap?: boolean;
};

export function AddressInput({
  id,
  value,
  onChange,
  onSelect,
  placeholder,
  disabled,
  invalid,
  showMap = false,
}: AddressInputProps) {
  const reactId = useId();
  const inputId = id ?? reactId;
  const configured = isMapboxConfigured();
  const debounced = useDebouncedValue(value, 300);
  const [suggestions, setSuggestions] = useState<GeocodeSuggestion[]>([]);
  const [open, setOpen] = useState(false);
  const [center, setCenter] = useState<[number, number] | null>(null);

  useEffect(() => {
    if (!configured) {
      return;
    }
    const controller = new AbortController();
    let active = true;

    const run = async () => {
      await Promise.resolve();
      if (!active) {
        return;
      }
      const query = debounced.trim();
      if (query.length < 3) {
        setSuggestions([]);
        setCenter(null);
        return;
      }
      try {
        const results = await geocodeAddress(query, controller.signal);
        if (!active) {
          return;
        }
        setSuggestions(results);
        setOpen(results.length > 0);
        if (showMap) {
          setCenter(results[0]?.center ?? null);
        }
      } catch {
        // Aborted or network failure: leave suggestions unchanged.
      }
    };

    void run();
    return () => {
      active = false;
      controller.abort();
    };
  }, [debounced, configured, showMap]);

  function select(suggestion: GeocodeSuggestion) {
    onChange(suggestion.placeName);
    onSelect?.(suggestion);
    setCenter(suggestion.center);
    setSuggestions([]);
    setOpen(false);
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="relative">
        <Input
          id={inputId}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onFocus={() => setOpen(suggestions.length > 0)}
          onBlur={() => setOpen(false)}
          placeholder={placeholder}
          disabled={disabled}
          aria-invalid={invalid || undefined}
          autoComplete="off"
        />
        {open ? (
          <ul className="absolute z-20 mt-1 w-full overflow-hidden rounded-lg border bg-popover text-sm shadow-md">
            {suggestions.map((suggestion) => (
              <li key={suggestion.id}>
                <button
                  type="button"
                  className="block w-full px-3 py-2 text-left hover:bg-muted"
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => select(suggestion)}
                >
                  {suggestion.placeName}
                </button>
              </li>
            ))}
          </ul>
        ) : null}
      </div>
      {showMap && center ? (
        <div
          role="img"
          aria-label="Map preview"
          className="h-40 w-full rounded-lg border bg-muted bg-cover bg-center"
          style={{ backgroundImage: `url(${staticMapUrl(center)})` }}
        />
      ) : null}
    </div>
  );
}
