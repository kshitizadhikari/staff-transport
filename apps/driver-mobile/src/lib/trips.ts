import { apiFetch } from '@/lib/api';
import type { Paginated, Trip } from '@/lib/types';

type ListParams = { status?: string; date?: string };

function query(params: ListParams = {}) {
  const search = new URLSearchParams();
  if (params.status) search.set('status', params.status);
  if (params.date) search.set('date', params.date);
  const value = search.toString();
  return value ? `?${value}` : '';
}

export const tripApi = {
  mine: (params: ListParams = {}) =>
    apiFetch<Paginated<Trip>>(`/me/trips${query(params)}`),
  mineById: (id: string) => apiFetch<Trip>(`/me/trips/${id}`),
  start: (id: string) =>
    apiFetch<Trip>(`/trips/${id}/start`, { method: 'POST' }),
  complete: (id: string) =>
    apiFetch<Trip>(`/trips/${id}/complete`, { method: 'POST' }),
  arrive: (id: string, stopId: string) =>
    apiFetch<Trip>(`/trips/${id}/stops/${stopId}/arrive`, { method: 'POST' }),
  depart: (id: string, stopId: string) =>
    apiFetch<Trip>(`/trips/${id}/stops/${stopId}/depart`, { method: 'POST' }),
  pickup: (id: string, passengerId: string) =>
    apiFetch<Trip>(`/trips/${id}/passengers/${passengerId}/pickup`, {
      method: 'POST',
    }),
  noShow: (id: string, passengerId: string) =>
    apiFetch<Trip>(`/trips/${id}/passengers/${passengerId}/no-show`, {
      method: 'POST',
    }),
};
