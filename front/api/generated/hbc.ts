/* eslint-disable */
/* tslint:disable */
// @ts-nocheck
/*
 * ---------------------------------------------------------------
 * ## THIS FILE WAS GENERATED VIA SWAGGER-TYPESCRIPT-API        ##
 * ##                                                           ##
 * ## AUTHOR: acacode                                           ##
 * ## SOURCE: https://github.com/acacode/swagger-typescript-api ##
 * ---------------------------------------------------------------
 */

/**
 * Стадия стартапа. Заполняется только для kind = STARTUP.
 * @default "STARTUP_STAGE_UNSPECIFIED"
 */
export enum V1StartupStage {
  STARTUP_STAGE_UNSPECIFIED = "STARTUP_STAGE_UNSPECIFIED",
  STARTUP_STAGE_IDEA = "STARTUP_STAGE_IDEA",
  STARTUP_STAGE_PROTOTYPE = "STARTUP_STAGE_PROTOTYPE",
  STARTUP_STAGE_MVP = "STARTUP_STAGE_MVP",
  STARTUP_STAGE_LAUNCHED = "STARTUP_STAGE_LAUNCHED",
}

/** @default "LISTING_STATUS_UNSPECIFIED" */
export enum V1ListingStatus {
  LISTING_STATUS_UNSPECIFIED = "LISTING_STATUS_UNSPECIFIED",
  LISTING_STATUS_ACTIVE = "LISTING_STATUS_ACTIVE",
  LISTING_STATUS_CLOSED = "LISTING_STATUS_CLOSED",
}

/** @default "LISTING_KIND_UNSPECIFIED" */
export enum V1ListingKind {
  LISTING_KIND_UNSPECIFIED = "LISTING_KIND_UNSPECIFIED",
  LISTING_KIND_STARTUP = "LISTING_KIND_STARTUP",
  LISTING_KIND_VACANCY = "LISTING_KIND_VACANCY",
}

/** @default "APPLICATION_STATUS_UNSPECIFIED" */
export enum V1ApplicationStatus {
  APPLICATION_STATUS_UNSPECIFIED = "APPLICATION_STATUS_UNSPECIFIED",
  APPLICATION_STATUS_PENDING = "APPLICATION_STATUS_PENDING",
  APPLICATION_STATUS_ACCEPTED = "APPLICATION_STATUS_ACCEPTED",
  APPLICATION_STATUS_DECLINED = "APPLICATION_STATUS_DECLINED",
  APPLICATION_STATUS_WITHDRAWN = "APPLICATION_STATUS_WITHDRAWN",
}

export type HseBusinessClubV1AcceptApplicationBody = object;

export type HseBusinessClubV1CloseListingBody = object;

export interface HseBusinessClubV1CreateApplicationBody {
  message?: string;
}

export type HseBusinessClubV1DeclineApplicationBody = object;

export interface HseBusinessClubV1UpdateListingBody {
  title?: string;
  description?: string;
  lookingFor?: string;
  tags?: string[];
  /** Стадия стартапа. Заполняется только для kind = STARTUP. */
  stage?: V1StartupStage;
  compensation?: string;
}

export type HseBusinessClubV1WithdrawApplicationBody = object;

/**
 * Message that represents an arbitrary HTTP body. It should only be used for
 * payload formats that can't be represented as JSON, such as raw binary or
 * an HTML page.
 *
 *
 * This message can be used both in streaming and non-streaming API methods in
 * the request as well as the response.
 *
 * It can be used as a top-level request field, which is convenient if one
 * wants to extract parameters from either the URL or HTTP template into the
 * request fields and also want access to the raw HTTP body.
 *
 * Example:
 *
 *     message GetResourceRequest {
 *       // A unique request id.
 *       string request_id = 1;
 *
 *       // The raw HTTP body is bound to this field.
 *       google.api.HttpBody http_body = 2;
 *
 *     }
 *
 *     service ResourceService {
 *       rpc GetResource(GetResourceRequest)
 *         returns (google.api.HttpBody);
 *       rpc UpdateResource(google.api.HttpBody)
 *         returns (google.protobuf.Empty);
 *
 *     }
 *
 * Example with streaming methods:
 *
 *     service CaldavService {
 *       rpc GetCalendar(stream google.api.HttpBody)
 *         returns (stream google.api.HttpBody);
 *       rpc UpdateCalendar(stream google.api.HttpBody)
 *         returns (stream google.api.HttpBody);
 *
 *     }
 *
 * Use of this type only changes how the request and response bodies are
 * handled, all other features will continue to work unchanged.
 */
export interface ApiHttpBody {
  /** The HTTP Content-Type header value specifying the content type of the body. */
  contentType?: string;
  /**
   * The HTTP request/response body as raw binary.
   * @format byte
   */
  data?: Blob;
  /**
   * Application specific response metadata. Must be set in the first response
   * for streaming APIs.
   */
  extensions?: ProtobufAny[];
}

/**
 * `Any` contains an arbitrary serialized protocol buffer message along with a
 * URL that describes the type of the serialized message.
 *
 * Protobuf library provides support to pack/unpack Any values in the form
 * of utility functions or additional generated methods of the Any type.
 *
 * Example 1: Pack and unpack a message in C++.
 *
 *     Foo foo = ...;
 *     Any any;
 *     any.PackFrom(foo);
 *     ...
 *     if (any.UnpackTo(&foo)) {
 *       ...
 *     }
 *
 * Example 2: Pack and unpack a message in Java.
 *
 *     Foo foo = ...;
 *     Any any = Any.pack(foo);
 *     ...
 *     if (any.is(Foo.class)) {
 *       foo = any.unpack(Foo.class);
 *     }
 *     // or ...
 *     if (any.isSameTypeAs(Foo.getDefaultInstance())) {
 *       foo = any.unpack(Foo.getDefaultInstance());
 *     }
 *
 *  Example 3: Pack and unpack a message in Python.
 *
 *     foo = Foo(...)
 *     any = Any()
 *     any.Pack(foo)
 *     ...
 *     if any.Is(Foo.DESCRIPTOR):
 *       any.Unpack(foo)
 *       ...
 *
 *  Example 4: Pack and unpack a message in Go
 *
 *      foo := &pb.Foo{...}
 *      any, err := anypb.New(foo)
 *      if err != nil {
 *        ...
 *      }
 *      ...
 *      foo := &pb.Foo{}
 *      if err := any.UnmarshalTo(foo); err != nil {
 *        ...
 *      }
 *
 * The pack methods provided by protobuf library will by default use
 * 'type.googleapis.com/full.type.name' as the type URL and the unpack
 * methods only use the fully qualified type name after the last '/'
 * in the type URL, for example "foo.bar.com/x/y.z" will yield type
 * name "y.z".
 *
 * JSON
 * ====
 * The JSON representation of an `Any` value uses the regular
 * representation of the deserialized, embedded message, with an
 * additional field `@type` which contains the type URL. Example:
 *
 *     package google.profile;
 *     message Person {
 *       string first_name = 1;
 *       string last_name = 2;
 *     }
 *
 *     {
 *       "@type": "type.googleapis.com/google.profile.Person",
 *       "firstName": <string>,
 *       "lastName": <string>
 *     }
 *
 * If the embedded message type is well-known and has a custom JSON
 * representation, that representation will be embedded adding a field
 * `value` which holds the custom JSON in addition to the `@type`
 * field. Example (for message [google.protobuf.Duration][]):
 *
 *     {
 *       "@type": "type.googleapis.com/google.protobuf.Duration",
 *       "value": "1.212s"
 *     }
 */
export interface ProtobufAny {
  /**
   * A URL/resource name that uniquely identifies the type of the serialized
   * protocol buffer message. This string must contain at least
   * one "/" character. The last segment of the URL's path must represent
   * the fully qualified name of the type (as in
   * `path/google.protobuf.Duration`). The name should be in a canonical form
   * (e.g., leading "." is not accepted).
   *
   * In practice, teams usually precompile into the binary all types that they
   * expect it to use in the context of Any. However, for URLs which use the
   * scheme `http`, `https`, or no scheme, one can optionally set up a type
   * server that maps type URLs to message definitions as follows:
   *
   * * If no scheme is provided, `https` is assumed.
   * * An HTTP GET on the URL must yield a [google.protobuf.Type][]
   *   value in binary format, or produce an error.
   * * Applications are allowed to cache lookup results based on the
   *   URL, or have them precompiled into a binary to avoid any
   *   lookup. Therefore, binary compatibility needs to be preserved
   *   on changes to types. (Use versioned type names to manage
   *   breaking changes.)
   *
   * Note: this functionality is not currently available in the official
   * protobuf release, and it is not used for type URLs beginning with
   * type.googleapis.com. As of May 2023, there are no widely used type server
   * implementations and no plans to implement one.
   *
   * Schemes other than `http`, `https` (or the empty scheme) might be
   * used with implementation specific semantics.
   */
  "@type"?: string;
  [key: string]: any;
}

export interface RpcStatus {
  /** @format int32 */
  code?: number;
  message?: string;
  details?: ProtobufAny[];
}

export interface V1Application {
  id?: string;
  /** Краткая публикация для вложения в отклик. */
  listing?: V1ListingSummary;
  applicant?: V1User;
  message?: string;
  status?: V1ApplicationStatus;
  /** @format date-time */
  createdAt?: string;
  /** @format date-time */
  updatedAt?: string;
  /**
   * Момент accept/decline. null для pending и withdrawn.
   * @format date-time
   */
  decidedAt?: string;
}

export interface V1Listing {
  id?: string;
  kind?: V1ListingKind;
  title?: string;
  description?: string;
  /** Кого ищет автор (свободный текст). */
  lookingFor?: string;
  /** Роли и навыки, которые ищет автор. Используются как теги в карточке. */
  tags?: string[];
  /** Только для STARTUP, иначе UNSPECIFIED. */
  stage?: V1StartupStage;
  /** Только для VACANCY (условия: доля, оплата, формат), иначе пусто. */
  compensation?: string;
  status?: V1ListingStatus;
  author?: V1User;
  /** @format date-time */
  createdAt?: string;
  /** @format date-time */
  updatedAt?: string;
  /**
   * null, пока публикация активна.
   * @format date-time
   */
  closedAt?: string;
  /** true, если текущий пользователь — автор. */
  isMine?: boolean;
  /** Не отозванный отклик текущего пользователя на эту публикацию. null, если нет. */
  myApplication?: V1Application;
  /** Только для is_mine = true, иначе null. */
  stats?: V1ListingStats;
}

/** Статистика откликов по публикации. Отдаётся только автору. */
export interface V1ListingStats {
  /** @format int32 */
  pending?: number;
  /** @format int32 */
  accepted?: number;
  /** @format int32 */
  declined?: number;
  /** @format int32 */
  total?: number;
}

/** Краткая публикация для вложения в отклик. */
export interface V1ListingSummary {
  id?: string;
  kind?: V1ListingKind;
  title?: string;
  status?: V1ListingStatus;
  author?: V1User;
}

export interface V1User {
  id?: string;
  username?: string;
  displayName?: string;
  headline?: string;
  about?: string;
  skills?: string[];
  telegram?: string;
  /** @format date-time */
  createdAt?: string;
  /**
   * Путь к фото относительно корня API: "/v1/avatars/<uuid>.jpg". Пусто — фото нет,
   * клиент рисует сгенерированный аватар по username.
   */
  avatarUrl?: string;
}

export interface V1V1ApplicationsResponse {
  items?: V1Application[];
}

export interface V1V1AuthResponse {
  token?: string;
  user?: V1User;
}

export interface V1V1CreateListingRequest {
  kind?: V1ListingKind;
  title?: string;
  description?: string;
  lookingFor?: string;
  tags?: string[];
  /** Стадия стартапа. Заполняется только для kind = STARTUP. */
  stage?: V1StartupStage;
  compensation?: string;
}

export interface V1V1ListListingsResponse {
  items?: V1Listing[];
  /** @format int32 */
  total?: number;
  /** @format int32 */
  page?: number;
  /** @format int32 */
  limit?: number;
}

export interface V1V1ListMyListingsResponse {
  items?: V1Listing[];
}

export interface V1V1LoginRequest {
  username?: string;
  password?: string;
}

export interface V1V1PingResponse {
  ok?: boolean;
}

export interface V1V1RegisterRequest {
  username?: string;
  password?: string;
  displayName?: string;
}

export interface V1V1UpdateMeRequest {
  displayName?: string;
  headline?: string;
  about?: string;
  skills?: string[];
  telegram?: string;
}

export interface V1V1UploadAvatarRequest {
  /**
   * Исходное изображение (в JSON — base64). JPEG, PNG или WebP, до 5 МБ.
   * @format byte
   */
  data?: Blob;
}

import type {
  AxiosInstance,
  AxiosRequestConfig,
  AxiosResponse,
  HeadersDefaults,
  ResponseType,
} from "axios";
import axios from "axios";

export type QueryParamsType = Record<string | number, any>;

export interface FullRequestParams
  extends Omit<AxiosRequestConfig, "data" | "params" | "url" | "responseType"> {
  /** set parameter to `true` for call `securityWorker` for this request */
  secure?: boolean;
  /** request path */
  path: string;
  /** content type of request body */
  type?: ContentType;
  /** query params */
  query?: QueryParamsType;
  /** format of response (i.e. response.json() -> format: "json") */
  format?: ResponseType;
  /** request body */
  body?: unknown;
}

export type RequestParams = Omit<
  FullRequestParams,
  "body" | "method" | "query" | "path"
>;

export interface ApiConfig<SecurityDataType = unknown>
  extends Omit<AxiosRequestConfig, "data" | "cancelToken"> {
  securityWorker?: (
    securityData: SecurityDataType | null,
  ) => Promise<AxiosRequestConfig | void> | AxiosRequestConfig | void;
  secure?: boolean;
  format?: ResponseType;
}

export enum ContentType {
  Json = "application/json",
  JsonApi = "application/vnd.api+json",
  FormData = "multipart/form-data",
  UrlEncoded = "application/x-www-form-urlencoded",
  Text = "text/plain",
}

export class HttpClient<SecurityDataType = unknown> {
  public instance: AxiosInstance;
  private securityData: SecurityDataType | null = null;
  private securityWorker?: ApiConfig<SecurityDataType>["securityWorker"];
  private secure?: boolean;
  private format?: ResponseType;

  constructor({
    securityWorker,
    secure,
    format,
    ...axiosConfig
  }: ApiConfig<SecurityDataType> = {}) {
    this.instance = axios.create({
      ...axiosConfig,
      baseURL: axiosConfig.baseURL || "",
    });
    this.secure = secure;
    this.format = format;
    this.securityWorker = securityWorker;
  }

  public setSecurityData = (data: SecurityDataType | null) => {
    this.securityData = data;
  };

  protected mergeRequestParams(
    params1: AxiosRequestConfig,
    params2?: AxiosRequestConfig,
  ): AxiosRequestConfig {
    const method = params1.method || (params2 && params2.method);

    return {
      ...this.instance.defaults,
      ...params1,
      ...(params2 || {}),
      headers: {
        ...((method &&
          this.instance.defaults.headers[
            method.toLowerCase() as keyof HeadersDefaults
          ]) ||
          {}),
        ...(params1.headers || {}),
        ...((params2 && params2.headers) || {}),
      },
    };
  }

  protected stringifyFormItem(formItem: unknown) {
    if (typeof formItem === "object" && formItem !== null) {
      return JSON.stringify(formItem);
    } else {
      return `${formItem}`;
    }
  }

  protected createFormData(input: Record<string, unknown>): FormData {
    if (input instanceof FormData) {
      return input;
    }
    return Object.keys(input || {}).reduce((formData, key) => {
      const property = input[key];
      const propertyContent: any[] =
        property instanceof Array ? property : [property];

      for (const formItem of propertyContent) {
        const isFileType = formItem instanceof Blob || formItem instanceof File;
        formData.append(
          key,
          isFileType ? formItem : this.stringifyFormItem(formItem),
        );
      }

      return formData;
    }, new FormData());
  }

  public request = async <T = any, _E = any>({
    secure,
    path,
    type,
    query,
    format,
    body,
    ...params
  }: FullRequestParams): Promise<AxiosResponse<T>> => {
    const secureParams =
      ((typeof secure === "boolean" ? secure : this.secure) &&
        this.securityWorker &&
        (await this.securityWorker(this.securityData))) ||
      {};
    const requestParams = this.mergeRequestParams(params, secureParams);
    const responseFormat = format || this.format || undefined;

    if (
      type === ContentType.FormData &&
      body &&
      body !== null &&
      typeof body === "object"
    ) {
      body = this.createFormData(body as Record<string, unknown>);
    }

    if (
      type === ContentType.Text &&
      body &&
      body !== null &&
      typeof body !== "string"
    ) {
      body = JSON.stringify(body);
    }

    return this.instance.request({
      ...requestParams,
      headers: {
        ...(requestParams.headers || {}),
        ...(type ? { "Content-Type": type } : {}),
      },
      params: query,
      responseType: responseFormat,
      data: body,
      url: path,
    });
  };
}

/**
 * @title hbc-back
 * @version 1.0
 *
 * API модуля стартапов и вакансий Бизнес-клуба НИУ ВШЭ
 */
export class Api<
  SecurityDataType extends unknown,
> extends HttpClient<SecurityDataType> {
  hseBusinessClub = {
    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1Ping
     * @request GET:/v1/ping
     * @secure
     */
    hseBusinessClubV1Ping: (params: RequestParams = {}) =>
      this.request<V1V1PingResponse, RpcStatus>({
        path: `/v1/ping`,
        method: "GET",
        secure: true,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1Register
     * @request POST:/v1/auth/register
     * @secure
     */
    hseBusinessClubV1Register: (
      body: V1V1RegisterRequest,
      params: RequestParams = {},
    ) =>
      this.request<V1V1AuthResponse, RpcStatus>({
        path: `/v1/auth/register`,
        method: "POST",
        body: body,
        secure: true,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1Login
     * @request POST:/v1/auth/login
     * @secure
     */
    hseBusinessClubV1Login: (
      body: V1V1LoginRequest,
      params: RequestParams = {},
    ) =>
      this.request<V1V1AuthResponse, RpcStatus>({
        path: `/v1/auth/login`,
        method: "POST",
        body: body,
        secure: true,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1GetMe
     * @request GET:/v1/me
     * @secure
     */
    hseBusinessClubV1GetMe: (params: RequestParams = {}) =>
      this.request<V1User, RpcStatus>({
        path: `/v1/me`,
        method: "GET",
        secure: true,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1UpdateMe
     * @summary Полная замена редактируемых полей профиля.
     * @request PUT:/v1/me
     * @secure
     */
    hseBusinessClubV1UpdateMe: (
      body: V1V1UpdateMeRequest,
      params: RequestParams = {},
    ) =>
      this.request<V1User, RpcStatus>({
        path: `/v1/me`,
        method: "PUT",
        body: body,
        secure: true,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1GetUser
     * @request GET:/v1/users/{id}
     * @secure
     */
    hseBusinessClubV1GetUser: (id: string, params: RequestParams = {}) =>
      this.request<V1User, RpcStatus>({
        path: `/v1/users/${id}`,
        method: "GET",
        secure: true,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1DeleteAvatar
     * @summary Удалить фото. Идемпотентно: без фото — 200 с текущим профилем.
     * @request DELETE:/v1/me/avatar
     * @secure
     */
    hseBusinessClubV1DeleteAvatar: (params: RequestParams = {}) =>
      this.request<V1User, RpcStatus>({
        path: `/v1/me/avatar`,
        method: "DELETE",
        secure: true,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1UploadAvatar
     * @summary Загрузить или заменить фото. Тело — JSON с изображением в base64 (JPEG/PNG/WebP, до 5 МБ). Сервер обрезает до квадрата, уменьшает до 512×512 и сохраняет как JPEG.
     * @request POST:/v1/me/avatar
     * @secure
     */
    hseBusinessClubV1UploadAvatar: (
      body: V1V1UploadAvatarRequest,
      params: RequestParams = {},
    ) =>
      this.request<V1User, RpcStatus>({
        path: `/v1/me/avatar`,
        method: "POST",
        body: body,
        secure: true,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1GetAvatar
     * @summary Отдать файл фото (image/jpeg) по пути из User.avatar_url. Без авторизации: имя файла — непредсказуемый uuid. Ответ кэшируется (immutable).
     * @request GET:/v1/avatars/{file}
     * @secure
     */
    hseBusinessClubV1GetAvatar: (file: string, params: RequestParams = {}) =>
      this.request<ApiHttpBody, RpcStatus>({
        path: `/v1/avatars/${file}`,
        method: "GET",
        secure: true,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1ListListings
     * @summary Лента: только активные публикации всех участников, новые первыми.
     * @request GET:/v1/listings
     * @secure
     */
    hseBusinessClubV1ListListings: (
      query?: {
        /**
         * UNSPECIFIED = все типы.
         * @default "LISTING_KIND_UNSPECIFIED"
         */
        kind?:
          | "LISTING_KIND_UNSPECIFIED"
          | "LISTING_KIND_STARTUP"
          | "LISTING_KIND_VACANCY";
        /** Поиск по title и description без учёта регистра. Пусто = без фильтра. */
        query?: string;
        /**
         * С 1. По умолчанию 1.
         * @format int32
         */
        page?: number;
        /**
         * 1..50. По умолчанию 20.
         * @format int32
         */
        limit?: number;
      },
      params: RequestParams = {},
    ) =>
      this.request<V1V1ListListingsResponse, RpcStatus>({
        path: `/v1/listings`,
        method: "GET",
        query: query,
        secure: true,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1CreateListing
     * @request POST:/v1/listings
     * @secure
     */
    hseBusinessClubV1CreateListing: (
      body: V1V1CreateListingRequest,
      params: RequestParams = {},
    ) =>
      this.request<V1Listing, RpcStatus>({
        path: `/v1/listings`,
        method: "POST",
        body: body,
        secure: true,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1GetListing
     * @request GET:/v1/listings/{id}
     * @secure
     */
    hseBusinessClubV1GetListing: (id: string, params: RequestParams = {}) =>
      this.request<V1Listing, RpcStatus>({
        path: `/v1/listings/${id}`,
        method: "GET",
        secure: true,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1UpdateListing
     * @summary Только автор, только пока публикация активна. Поле kind менять нельзя.
     * @request PUT:/v1/listings/{id}
     * @secure
     */
    hseBusinessClubV1UpdateListing: (
      id: string,
      body: HseBusinessClubV1UpdateListingBody,
      params: RequestParams = {},
    ) =>
      this.request<V1Listing, RpcStatus>({
        path: `/v1/listings/${id}`,
        method: "PUT",
        body: body,
        secure: true,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1CloseListing
     * @summary Только автор. Идемпотентно: повторное закрытие возвращает 200.
     * @request POST:/v1/listings/{id}/close
     * @secure
     */
    hseBusinessClubV1CloseListing: (
      id: string,
      body: HseBusinessClubV1CloseListingBody,
      params: RequestParams = {},
    ) =>
      this.request<V1Listing, RpcStatus>({
        path: `/v1/listings/${id}/close`,
        method: "POST",
        body: body,
        secure: true,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1ListMyListings
     * @summary Публикации текущего пользователя (активные и закрытые) со статистикой откликов.
     * @request GET:/v1/me/listings
     * @secure
     */
    hseBusinessClubV1ListMyListings: (
      query?: {
        /**
         * UNSPECIFIED = все.
         * @default "LISTING_STATUS_UNSPECIFIED"
         */
        status?:
          | "LISTING_STATUS_UNSPECIFIED"
          | "LISTING_STATUS_ACTIVE"
          | "LISTING_STATUS_CLOSED";
      },
      params: RequestParams = {},
    ) =>
      this.request<V1V1ListMyListingsResponse, RpcStatus>({
        path: `/v1/me/listings`,
        method: "GET",
        query: query,
        secure: true,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1ListListingApplications
     * @summary Отклики на конкретную публикацию. Только автор публикации.
     * @request GET:/v1/listings/{listingId}/applications
     * @secure
     */
    hseBusinessClubV1ListListingApplications: (
      listingId: string,
      query?: {
        /**
         * UNSPECIFIED = все.
         * @default "APPLICATION_STATUS_UNSPECIFIED"
         */
        status?:
          | "APPLICATION_STATUS_UNSPECIFIED"
          | "APPLICATION_STATUS_PENDING"
          | "APPLICATION_STATUS_ACCEPTED"
          | "APPLICATION_STATUS_DECLINED"
          | "APPLICATION_STATUS_WITHDRAWN";
      },
      params: RequestParams = {},
    ) =>
      this.request<V1V1ApplicationsResponse, RpcStatus>({
        path: `/v1/listings/${listingId}/applications`,
        method: "GET",
        query: query,
        secure: true,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1CreateApplication
     * @summary Откликнуться на публикацию. Нельзя на свою, нельзя на закрытую, нельзя повторно, пока есть не отозванный отклик (409).
     * @request POST:/v1/listings/{listingId}/applications
     * @secure
     */
    hseBusinessClubV1CreateApplication: (
      listingId: string,
      body: HseBusinessClubV1CreateApplicationBody,
      params: RequestParams = {},
    ) =>
      this.request<V1Application, RpcStatus>({
        path: `/v1/listings/${listingId}/applications`,
        method: "POST",
        body: body,
        secure: true,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1AcceptApplication
     * @summary Автор публикации принимает отклик. pending -> accepted. Идемпотентно для accepted.
     * @request POST:/v1/applications/{id}/accept
     * @secure
     */
    hseBusinessClubV1AcceptApplication: (
      id: string,
      body: HseBusinessClubV1AcceptApplicationBody,
      params: RequestParams = {},
    ) =>
      this.request<V1Application, RpcStatus>({
        path: `/v1/applications/${id}/accept`,
        method: "POST",
        body: body,
        secure: true,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1DeclineApplication
     * @summary Автор публикации отклоняет отклик. pending -> declined. Идемпотентно для declined.
     * @request POST:/v1/applications/{id}/decline
     * @secure
     */
    hseBusinessClubV1DeclineApplication: (
      id: string,
      body: HseBusinessClubV1DeclineApplicationBody,
      params: RequestParams = {},
    ) =>
      this.request<V1Application, RpcStatus>({
        path: `/v1/applications/${id}/decline`,
        method: "POST",
        body: body,
        secure: true,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1WithdrawApplication
     * @summary Откликнувшийся отзывает свой отклик. pending -> withdrawn. Идемпотентно для withdrawn.
     * @request POST:/v1/applications/{id}/withdraw
     * @secure
     */
    hseBusinessClubV1WithdrawApplication: (
      id: string,
      body: HseBusinessClubV1WithdrawApplicationBody,
      params: RequestParams = {},
    ) =>
      this.request<V1Application, RpcStatus>({
        path: `/v1/applications/${id}/withdraw`,
        method: "POST",
        body: body,
        secure: true,
        type: ContentType.Json,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1ListSentApplications
     * @summary Отклики, отправленные текущим пользователем.
     * @request GET:/v1/me/applications/sent
     * @secure
     */
    hseBusinessClubV1ListSentApplications: (
      query?: {
        /**
         * UNSPECIFIED = все.
         * @default "APPLICATION_STATUS_UNSPECIFIED"
         */
        status?:
          | "APPLICATION_STATUS_UNSPECIFIED"
          | "APPLICATION_STATUS_PENDING"
          | "APPLICATION_STATUS_ACCEPTED"
          | "APPLICATION_STATUS_DECLINED"
          | "APPLICATION_STATUS_WITHDRAWN";
      },
      params: RequestParams = {},
    ) =>
      this.request<V1V1ApplicationsResponse, RpcStatus>({
        path: `/v1/me/applications/sent`,
        method: "GET",
        query: query,
        secure: true,
        format: "json",
        ...params,
      }),

    /**
     * No description
     *
     * @tags HseBusinessClub
     * @name HseBusinessClubV1ListReceivedApplications
     * @summary Отклики, полученные на все публикации текущего пользователя.
     * @request GET:/v1/me/applications/received
     * @secure
     */
    hseBusinessClubV1ListReceivedApplications: (
      query?: {
        /**
         * UNSPECIFIED = все.
         * @default "APPLICATION_STATUS_UNSPECIFIED"
         */
        status?:
          | "APPLICATION_STATUS_UNSPECIFIED"
          | "APPLICATION_STATUS_PENDING"
          | "APPLICATION_STATUS_ACCEPTED"
          | "APPLICATION_STATUS_DECLINED"
          | "APPLICATION_STATUS_WITHDRAWN";
      },
      params: RequestParams = {},
    ) =>
      this.request<V1V1ApplicationsResponse, RpcStatus>({
        path: `/v1/me/applications/received`,
        method: "GET",
        query: query,
        secure: true,
        format: "json",
        ...params,
      }),
  };
}
