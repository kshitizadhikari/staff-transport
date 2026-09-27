import { useFocusEffect, useRouter } from 'expo-router';
import { useCallback, useState } from 'react';
import { Pressable, ScrollView, StyleSheet } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { BottomTabInset, MaxContentWidth, Spacing } from '@/constants/theme';
import { ApiError } from '@/lib/api';
import { tripApi } from '@/lib/trips';
import type { Trip } from '@/lib/types';

export default function TodayScreen() {
  const router = useRouter();
  const [trips, setTrips] = useState<Trip[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setError(null);
      const result = await tripApi.mine();
      setTrips(result.data);
    } catch (cause) {
      setError(
        cause instanceof ApiError ? cause.message : 'Unable to load trips.',
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useFocusEffect(
    useCallback(() => {
      setLoading(true);
      void load();
    }, [load]),
  );

  return (
    <ThemedView style={styles.container}>
      <SafeAreaView style={styles.safeArea}>
        <ScrollView contentContainerStyle={styles.content}>
          <ThemedText type="subtitle">Today</ThemedText>
          <ThemedText type="small" themeColor="textSecondary">
            Trips assigned to you
          </ThemedText>

          {loading ? (
            <ThemedText type="small" themeColor="textSecondary">
              Loading…
            </ThemedText>
          ) : null}
          {error ? (
            <ThemedText type="small" style={styles.error}>
              {error}
            </ThemedText>
          ) : null}
          {!loading && !error && trips.length === 0 ? (
            <ThemedText type="small" themeColor="textSecondary">
              No trips assigned.
            </ThemedText>
          ) : null}

          {trips.map((trip) => (
            <Pressable
              key={trip.id}
              onPress={() =>
                router.push({ pathname: '/trip/[id]', params: { id: trip.id } })
              }>
              <ThemedView type="backgroundElement" style={styles.card}>
                <ThemedText type="smallBold">
                  {trip.type.replace(/_/g, ' ')}
                </ThemedText>
                <ThemedText type="small" themeColor="textSecondary">
                  {formatWhen(trip.scheduled_start_at)} ·{' '}
                  {trip.status.replace(/_/g, ' ')}
                </ThemedText>
                <ThemedText type="small" themeColor="textSecondary">
                  {trip.vehicle_registration ?? 'Vehicle not assigned'}
                </ThemedText>
              </ThemedView>
            </Pressable>
          ))}
        </ScrollView>
      </SafeAreaView>
    </ThemedView>
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
    paddingBottom: BottomTabInset + Spacing.four,
    gap: Spacing.three,
  },
  card: {
    padding: Spacing.three,
    borderRadius: Spacing.three,
    gap: Spacing.one,
  },
  error: {
    color: '#d92d20',
  },
});
