import { useFocusEffect, useLocalSearchParams, useRouter } from 'expo-router';
import { useCallback, useState } from 'react';
import { Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { MaxContentWidth, Spacing } from '@/constants/theme';
import { ApiError } from '@/lib/api';
import { tripApi } from '@/lib/trips';
import type { Trip } from '@/lib/types';

export default function TripScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const router = useRouter();
  const [trip, setTrip] = useState<Trip | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setError(null);
      setTrip(await tripApi.mineById(id));
    } catch (cause) {
      setError(
        cause instanceof ApiError ? cause.message : 'Unable to load trip.',
      );
    } finally {
      setLoading(false);
    }
  }, [id]);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load]),
  );

  const run = useCallback(async (action: () => Promise<Trip>) => {
    setBusy(true);
    setError(null);
    try {
      setTrip(await action());
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : 'Action failed.');
    } finally {
      setBusy(false);
    }
  }, []);

  if (loading) {
    return (
      <ThemedView style={styles.container}>
        <SafeAreaView style={styles.safeArea}>
          <ThemedText type="small" themeColor="textSecondary">
            Loading…
          </ThemedText>
        </SafeAreaView>
      </ThemedView>
    );
  }

  if (!trip) {
    return (
      <ThemedView style={styles.container}>
        <SafeAreaView style={styles.safeArea}>
          <Pressable onPress={() => router.back()}>
            <ThemedText type="link" themeColor="textSecondary">
              ‹ Back
            </ThemedText>
          </Pressable>
          <ThemedText type="small" style={styles.error}>
            {error ?? 'Trip not found.'}
          </ThemedText>
        </SafeAreaView>
      </ThemedView>
    );
  }

  const currentStop = trip.stops?.find(
    (stop) => stop.status !== 'departed' && stop.status !== 'skipped',
  );

  return (
    <ThemedView style={styles.container}>
      <SafeAreaView style={styles.safeArea}>
        <ScrollView contentContainerStyle={styles.content}>
          <Pressable onPress={() => router.back()}>
            <ThemedText type="link" themeColor="textSecondary">
              ‹ Back
            </ThemedText>
          </Pressable>

          <ThemedText type="subtitle">
            {trip.type.replace(/_/g, ' ')}
          </ThemedText>
          <ThemedText type="small" themeColor="textSecondary">
            {formatWhen(trip.scheduled_start_at)} ·{' '}
            {trip.status.replace(/_/g, ' ')}
          </ThemedText>
          <ThemedText type="small" themeColor="textSecondary">
            {trip.vehicle_registration ?? 'Vehicle not assigned'}
          </ThemedText>

          {error ? (
            <ThemedText type="small" style={styles.error}>
              {error}
            </ThemedText>
          ) : null}

          {trip.status === 'assigned' ? (
            <Action
              label={busy ? 'Starting…' : 'Start trip'}
              primary
              disabled={busy}
              onPress={() => run(() => tripApi.start(trip.id))}
            />
          ) : null}

          <ThemedText type="smallBold">Stops</ThemedText>
          {(trip.stops ?? []).map((stop) => (
            <ThemedView
              key={stop.id}
              type="backgroundElement"
              style={styles.card}>
              <ThemedText type="smallBold">
                {stop.sequence + 1}. {stop.type} · {stop.status}
              </ThemedText>
              <ThemedText type="small" themeColor="textSecondary">
                {stop.address ?? 'No address'}
              </ThemedText>
              {trip.status === 'in_progress' &&
              currentStop?.id === stop.id &&
              stop.status === 'pending' ? (
                <Action
                  label="Arrived"
                  disabled={busy}
                  onPress={() => run(() => tripApi.arrive(trip.id, stop.id))}
                />
              ) : null}
              {trip.status === 'in_progress' && stop.status === 'arrived' ? (
                <Action
                  label="Depart"
                  disabled={busy}
                  onPress={() => run(() => tripApi.depart(trip.id, stop.id))}
                />
              ) : null}
            </ThemedView>
          ))}

          <ThemedText type="smallBold">Passengers</ThemedText>
          {(trip.passengers ?? []).map((passenger) => (
            <ThemedView
              key={passenger.id}
              type="backgroundElement"
              style={styles.card}>
              <ThemedText type="smallBold">
                {passenger.name} · {passenger.status.replace(/_/g, ' ')}
              </ThemedText>
              {trip.status === 'in_progress' &&
              passenger.status === 'assigned' ? (
                <View style={styles.actionsRow}>
                  <Action
                    label="Picked up"
                    disabled={busy}
                    onPress={() =>
                      run(() => tripApi.pickup(trip.id, passenger.id))
                    }
                  />
                  <Action
                    label="No show"
                    disabled={busy}
                    onPress={() =>
                      run(() => tripApi.noShow(trip.id, passenger.id))
                    }
                  />
                </View>
              ) : null}
            </ThemedView>
          ))}

          {trip.status === 'in_progress' ? (
            <Action
              label={busy ? 'Working…' : 'Complete trip'}
              primary
              disabled={busy}
              onPress={() => run(() => tripApi.complete(trip.id))}
            />
          ) : null}
        </ScrollView>
      </SafeAreaView>
    </ThemedView>
  );
}

function Action({
  label,
  onPress,
  primary = false,
  disabled = false,
}: {
  label: string;
  onPress: () => void;
  primary?: boolean;
  disabled?: boolean;
}) {
  return (
    <Pressable onPress={onPress} disabled={disabled} style={styles.actionWrapper}>
      <ThemedView
        type={primary ? 'backgroundSelected' : 'backgroundElement'}
        style={[styles.action, disabled && styles.actionDisabled]}>
        <ThemedText type="smallBold">{label}</ThemedText>
      </ThemedView>
    </Pressable>
  );
}

function formatWhen(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  });
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    flexDirection: 'row',
    justifyContent: 'center',
  },
  safeArea: {
    flex: 1,
    maxWidth: MaxContentWidth,
    width: '100%',
  },
  content: {
    paddingHorizontal: Spacing.four,
    paddingVertical: Spacing.four,
    gap: Spacing.three,
  },
  card: {
    padding: Spacing.three,
    borderRadius: Spacing.three,
    gap: Spacing.two,
  },
  actionsRow: {
    flexDirection: 'row',
    gap: Spacing.two,
  },
  actionWrapper: {
    flex: 1,
  },
  action: {
    minHeight: 52,
    borderRadius: Spacing.three,
    alignItems: 'center',
    justifyContent: 'center',
  },
  actionDisabled: {
    opacity: 0.5,
  },
  error: {
    color: '#d92d20',
  },
});
