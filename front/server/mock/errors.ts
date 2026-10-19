/**
 * Errors in the grpc-gateway shape: `{ code, message, details: [] }` with the HTTP status
 * from the gRPC → HTTP mapping (SPEC §4.2).
 */
export const GrpcCode = {
  InvalidArgument: 3,
  NotFound: 5,
  AlreadyExists: 6,
  PermissionDenied: 7,
  FailedPrecondition: 9,
  Unimplemented: 12,
  Internal: 13,
  Unauthenticated: 16,
} as const;

export class MockError extends Error {
  constructor(
    readonly status: number,
    readonly code: number,
    message: string,
  ) {
    super(message);
    this.name = 'MockError';
  }

  toBody() {
    return { code: this.code, message: this.message, details: [] as unknown[] };
  }
}

export const invalidArgument = (message: string) => new MockError(400, GrpcCode.InvalidArgument, message);
export const failedPrecondition = (message: string) => new MockError(400, GrpcCode.FailedPrecondition, message);
export const unauthenticated = (message = 'Требуется авторизация') => new MockError(401, GrpcCode.Unauthenticated, message);
export const permissionDenied = (message = 'Недостаточно прав') => new MockError(403, GrpcCode.PermissionDenied, message);
export const notFound = (message = 'Не найдено') => new MockError(404, GrpcCode.NotFound, message);
export const alreadyExists = (message: string) => new MockError(409, GrpcCode.AlreadyExists, message);
export const methodNotAllowed = () => new MockError(405, GrpcCode.Unimplemented, 'Метод не поддерживается');
export const internal = () => new MockError(500, GrpcCode.Internal, 'Внутренняя ошибка');
