import http from 'node:http';
import { randomUUID } from 'node:crypto';

const SOCKET_PATH = '/var/run/trim_open_gateway_apiscope.socket';
const API_PATH = '/api/v1/trimapp';
const MAX_RESPONSE_BYTES = 1024 * 1024;

// Keep this list limited to methods actually used by your application.
const ALLOWED_REQUESTS = new Set([
  'trim.system.getPlatformConfig',
  'trim.file.getSharedAccessibleFolders',
  'trim.file.delSharedAccessibleFolder',
  'trim.file.getUserAccessibleFolders',
  'trim.file.delUserAccessibleFolder',
  'trim.file.checkUserACL',
  'trim.file.convertPath',
]);

export class TrimApiError extends Error {
  constructor(message, details = {}) {
    super(message);
    this.name = 'TrimApiError';
    this.httpStatus = details.httpStatus;
    this.code = details.code;
    this.reqId = details.reqId;
  }
}

/**
 * Call a fnOS backend Open API through the system Unix Socket.
 * Never pass an arbitrary req value from browser input.
 */
export async function callTrimApi({ appName, req, data = {}, timeoutMs = 8000 }) {
  if (typeof appName !== 'string' || appName.length === 0) {
    throw new TypeError('appName must be a non-empty string');
  }
  if (!ALLOWED_REQUESTS.has(req)) {
    throw new TypeError(`Unsupported or non-whitelisted req: ${req}`);
  }
  if (data === null || typeof data !== 'object' || Array.isArray(data)) {
    throw new TypeError('data must be an object');
  }

  // Read the current token for every call. Do not cache or persist it.
  const token = process.env.TRIM_API_TOKEN;
  if (!token) {
    throw new TrimApiError('TRIM_API_TOKEN is unavailable');
  }

  const reqId = randomUUID();
  const payload = JSON.stringify({ reqId, req, appName, data });

  return new Promise((resolve, reject) => {
    const request = http.request(
      {
        socketPath: SOCKET_PATH,
        path: API_PATH,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Content-Length': Buffer.byteLength(payload),
          Authorization: `Bearer ${token}`,
        },
      },
      (response) => {
        const chunks = [];
        let size = 0;

        response.on('data', (chunk) => {
          size += chunk.length;
          if (size > MAX_RESPONSE_BYTES) {
            request.destroy(new TrimApiError('Trim API response is too large', { reqId }));
            return;
          }
          chunks.push(chunk);
        });

        response.on('end', () => {
          const text = Buffer.concat(chunks).toString('utf8');
          let body;
          try {
            body = text ? JSON.parse(text) : {};
          } catch {
            reject(new TrimApiError('Trim API returned invalid JSON', {
              httpStatus: response.statusCode,
              reqId,
            }));
            return;
          }

          if ((response.statusCode ?? 500) < 200 || (response.statusCode ?? 500) >= 300) {
            reject(new TrimApiError(body.msg || 'Trim API HTTP error', {
              httpStatus: response.statusCode,
              code: body.code,
              reqId: body.reqId || reqId,
            }));
            return;
          }

          if (body.code !== 0) {
            reject(new TrimApiError(body.msg || 'Trim API business error', {
              httpStatus: response.statusCode,
              code: body.code,
              reqId: body.reqId || reqId,
            }));
            return;
          }

          resolve(body.data);
        });
      },
    );

    request.setTimeout(timeoutMs, () => {
      request.destroy(new TrimApiError('Trim API request timed out', { reqId }));
    });
    request.on('error', reject);
    request.end(payload);
  });
}
