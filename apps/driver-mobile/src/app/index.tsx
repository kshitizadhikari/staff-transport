import { Link } from 'expo-router';
import { Pressable, ScrollView, StyleSheet } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { BottomTabInset, MaxContentWidth, Spacing } from '@/constants/theme';
import type { TripSummary } from '@/lib/types';

const sampleTrips: TripSummary[] = [
  {
    id: 'demo-1',
    type: 'staff_pickup',
    status: 'assigned',
    scheduledStartAt: '2026-09-21T08:00:00Z',
    vehicle: 'BA 1 JA 2345',
    stopCount: 3,
    passengerCount: 4,
  },
  {
    id: 'demo-2',
    type: 'office_to_event',
    status: 'scheduled',
    scheduledStartAt: '2026-09-21T13:30:00Z',
    vehicle: null,
    stopCount: 2,
    passengerCount: 6,
  },
];

export default function TodayScreen() {
  return (
    <ThemedView style={styles.container}>
      <SafeAreaView style={styles.safeArea}>
        <ScrollView contentContainerStyle={styles.content}>
          <ThemedText type="subtitle">Today</ThemedText>
          <ThemedText type="small" themeColor="textSecondary">
            Trips assigned to you. Loaded from the API once authentication is
            implemented.
          </ThemedText>

          {sampleTrips.map((trip) => (
            <Link key={trip.id} href="/trip" asChild>
              <Pressable>
                <ThemedView type="backgroundElement" style={styles.card}>
                  <ThemedText type="smallBold">{trip.type}</ThemedText>
                  <ThemedText type="small" themeColor="textSecondary">
                    {trip.status} · {trip.stopCount} stops ·{' '}
                    {trip.passengerCount} passengers
                  </ThemedText>
                  <ThemedText type="small" themeColor="textSecondary">
                    {trip.vehicle ?? 'Vehicle not assigned'}
                  </ThemedText>
                </ThemedView>
              </Pressable>
            </Link>
          ))}
        </ScrollView>
      </SafeAreaView>
    </ThemedView>
  );
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
});
