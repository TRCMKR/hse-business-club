import type { Application } from '../types';
import type { V1ApplicationStatus } from '../generated/hbc';
import { BaseService, cleanQuery } from './base';

export class ApplicationsService extends BaseService {
  /** POST /v1/listings/{listingId}/applications — 409 if already applied. */
  async create(listingId: string, message: string): Promise<Application> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1CreateApplication(listingId, { message });
    return data as Application;
  }

  /** GET /v1/listings/{listingId}/applications — author only; pending first. */
  async listForListing(listingId: string, status?: V1ApplicationStatus): Promise<Application[]> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1ListListingApplications(
      listingId,
      cleanQuery({ status }),
    );
    return (data.items ?? []) as Application[];
  }

  /** GET /v1/me/applications/sent */
  async listSent(status?: V1ApplicationStatus): Promise<Application[]> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1ListSentApplications(cleanQuery({ status }));
    return (data.items ?? []) as Application[];
  }

  /** GET /v1/me/applications/received — pending first. */
  async listReceived(status?: V1ApplicationStatus): Promise<Application[]> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1ListReceivedApplications(cleanQuery({ status }));
    return (data.items ?? []) as Application[];
  }

  /** POST /v1/applications/{id}/accept — listing author; idempotent. */
  async accept(id: string): Promise<Application> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1AcceptApplication(id, {});
    return data as Application;
  }

  /** POST /v1/applications/{id}/decline — listing author; idempotent. */
  async decline(id: string): Promise<Application> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1DeclineApplication(id, {});
    return data as Application;
  }

  /** POST /v1/applications/{id}/withdraw — applicant; idempotent. */
  async withdraw(id: string): Promise<Application> {
    const { data } = await this.api.hseBusinessClub.hseBusinessClubV1WithdrawApplication(id, {});
    return data as Application;
  }
}
