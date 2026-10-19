import type { Listing, ListingInput, ListingsPage, ListingUpdateInput } from '../types';
import type { V1ListingKind, V1ListingStatus } from '../generated/hbc';
import { BaseService, cleanQuery } from './base';

export interface ListListingsParams {
  /** Omit or UNSPECIFIED = all kinds. */
  kind?: V1ListingKind
  query?: string
  /** From 1, default 1. */
  page?: number
  /** 1..50, default 20. */
  limit?: number
}

export class ListingsService extends BaseService {
  /** GET /v1/listings — feed (active only, newest first). */
  async list(params: ListListingsParams = {}): Promise<ListingsPage> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1ListListings(cleanQuery({ ...params }));
    return data as ListingsPage;
  }

  /** GET /v1/listings/{id} — active and closed. */
  async get(id: string): Promise<Listing> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1GetListing(id);
    return data as Listing;
  }

  /** POST /v1/listings */
  async create(input: ListingInput): Promise<Listing> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1CreateListing(input);
    return data as Listing;
  }

  /** PUT /v1/listings/{id} — author only, active only; kind can't change. */
  async update(id: string, input: ListingUpdateInput): Promise<Listing> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1UpdateListing(id, input);
    return data as Listing;
  }

  /** POST /v1/listings/{id}/close — idempotent. */
  async close(id: string): Promise<Listing> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1CloseListing(id, {});
    return data as Listing;
  }

  /** GET /v1/me/listings — my listings with `stats`. */
  async listMine(status?: V1ListingStatus): Promise<Listing[]> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1ListMyListings(cleanQuery({ status }));
    return (data.items ?? []) as Listing[];
  }
}
