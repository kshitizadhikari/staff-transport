import { Pressable, ScrollView, StyleSheet } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { BottomTabInset, MaxContentWidth, Spacing } from '@/constants/theme';

const stops = [
  { id: 's1', label: 'Stop 1 · Lakeside, Pokhara', kind: 'pickup' },
  { id: 's2', label: 'Stop 2 · Baneshwor, Kathmandu', kind: 'pickup' },
  { id: 's3', label: 'Office · Durbar Marg', kind: 'dropoff' },
];

export default function TripScreen() {
  return (
    <ThemedView style={styles.container}>
      <SafeAreaView style={styles.safeArea}>
        <ScrollView contentContainerStyle={styles.content}>
          <ThemedText type="subtitle">Current trip</ThemedText>
          <ThemedText type="small" themeColor="textSecondary">
            Ordered stops and passenger outcomes. Wired to the driver execution
            API in a later step.
          </ThemedText>

          {stops.map((stop) => (
            <ThemedView key={stop.id} type="backgroundElement" style={styles.row}>
              <ThemedText type="smallBold">{stop.kind}</ThemedText>
              <ThemedText type="small" themeColor="textSecondary">
                {stop.label}
              </ThemedText>
            </ThemedView>
          ))}

          <ThemedView style={styles.actions}>
            <ActionButton label="Start trip" primary />
            <ActionButton label="Picked up" />
            <ActionButton label="No show" />
            <ActionButton label="Complete trip" />
          </ThemedView>
        </ScrollView>
      </SafeAreaView>
    </ThemedView>
  );
}

function ActionButton({
  label,
  primary = false,
}: {
  label: string;
  primary?: boolean;
}) {
  return (
    <Pressable style={styles.buttonWrapper}>
      <ThemedView
        type={primary ? 'backgroundSelected' : 'backgroundElement'}
        style={styles.button}>
        <ThemedText type="smallBold">{label}</ThemedText>
      </ThemedView>
    </Pressable>
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
  row: {
    padding: Spacing.three,
    borderRadius: Spacing.three,
    gap: Spacing.one,
  },
  actions: {
    gap: Spacing.two,
    marginTop: Spacing.two,
  },
  buttonWrapper: {
    alignSelf: 'stretch',
  },
  button: {
    minHeight: 56,
    borderRadius: Spacing.three,
    alignItems: 'center',
    justifyContent: 'center',
  },
});
