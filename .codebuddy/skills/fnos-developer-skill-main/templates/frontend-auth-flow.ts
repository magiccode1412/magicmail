import { TrimApp } from '@trimjs/web-app';

const sdk = new TrimApp();
const APP_NAME = 'your-app';
const CALLBACK_PATH = `/app/${APP_NAME}/callback.html`;
const STATE_KEY = `${APP_NAME}:auth-state`;

function createAuthState(): string {
  const state = crypto.randomUUID();
  sessionStorage.setItem(STATE_KEY, state);
  return state;
}

function consumeAndValidateState(returnedState?: string): boolean {
  const expected = sessionStorage.getItem(STATE_KEY);
  sessionStorage.removeItem(STATE_KEY);
  return Boolean(expected && returnedState && expected === returnedState);
}

export async function initializeHostUi(): Promise<void> {
  const config = await sdk.getPlatformConfig();
  document.documentElement.lang = config.language;
  document.documentElement.dataset.theme = config.theme;

  if (sdk.isWeb === true && sdk.isStandaloneWeb === false) {
    await sdk.$on('os/theme', (theme) => {
      document.documentElement.dataset.theme = theme;
    });
    await sdk.$on('os/language', (language) => {
      document.documentElement.lang = language;
    });
  }
}

// Call this only from a direct user action such as a button click.
export async function requestUserDirectory(): Promise<string[]> {
  if (sdk.isStandaloneWeb) {
    const state = createAuthState();
    await sdk.openAppAuth(
      'pickUserFile',
      {
        appName: APP_NAME,
        directory: true,
        sidebarGroup: ['myFiles', 'otherShare', 'favorites'],
        redirectUri: CALLBACK_PATH,
        state,
      },
      {
        target: '_blank',
        features: 'width=750,height=630',
      },
    );
    return [];
  }

  const result = await sdk.pickUserFile({
    directory: true,
    title: '选择授权目录',
    okText: '确认授权',
    sidebarGroup: ['myFiles', 'otherShare', 'favorites'],
  });

  if (!result || result.code !== 0) {
    throw new Error(result?.msg || '目录授权未完成');
  }
  return result.data ?? [];
}

// Run this only on the same-origin callback page.
export function handleAuthCallback(): void {
  const result = sdk.parseAppAuthCallback(window.location.href);
  const returnedState = typeof result?.state === 'string' ? result.state : undefined;

  if (!consumeAndValidateState(returnedState)) {
    document.body.textContent = '授权回调校验失败，请返回应用重试。';
    return;
  }

  if (window.opener && !window.opener.closed) {
    window.opener.postMessage(
      {
        type: `${APP_NAME}:auth-result`,
        result,
      },
      window.location.origin,
    );
  }

  window.close();
}

export function listenForAuthResult(onRefresh: () => void): () => void {
  const listener = (event: MessageEvent) => {
    if (event.origin !== window.location.origin) return;
    if (event.data?.type !== `${APP_NAME}:auth-result`) return;
    onRefresh();
  };
  window.addEventListener('message', listener);
  return () => window.removeEventListener('message', listener);
}
