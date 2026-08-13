import createClient from "openapi-fetch";

import type { components, paths } from "@/api/generated/schema";

type ErrorResponse = components["schemas"]["ErrorResponse"];

const baseUrl = (
  import.meta.env.VITE_API_URL ?? "http://localhost:8080"
).replace(/\/$/, "");

export const apiClient = createClient<paths>({
  baseUrl,
  fetch: (request) => globalThis.fetch(request),
});

export class ApiError extends Error {
  readonly status: number;
  readonly requestID?: string;
  readonly details?: ErrorResponse;

  constructor(response: Response, body: unknown) {
    const details = isErrorResponse(body) ? body : undefined;
    super(
      details?.message ??
        `The API request failed with status ${response.status}.`,
    );
    this.name = "ApiError";
    this.status = response.status;
    this.requestID =
      details?.request_id ?? response.headers.get("X-Request-ID") ?? undefined;
    this.details = details;
  }
}

export function apiError(response: Response, body: unknown): ApiError {
  return new ApiError(response, body);
}

function isErrorResponse(value: unknown): value is ErrorResponse {
  if (typeof value !== "object" || value === null) {
    return false;
  }

  const candidate = value as Record<string, unknown>;
  return (
    typeof candidate.code === "string" &&
    typeof candidate.message === "string" &&
    typeof candidate.request_id === "string"
  );
}
