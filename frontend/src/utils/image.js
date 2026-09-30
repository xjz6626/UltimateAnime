import { isWebMode } from '../api';

export function proxyImg(url) {
  if (!url) return '';
  if (url.startsWith('http://127.0.0.1:54321/img?')) {
    return isWebMode() ? url.replace('http://127.0.0.1:54321', '') : url;
  }
  const httpsUrl = url.replace(/^http:\/\//i, 'https://');
  if (!/^https?:\/\//i.test(httpsUrl)) return httpsUrl;
  const path = isWebMode() ? '/img' : 'http://127.0.0.1:54321/img';
  return `${path}?u=${encodeURIComponent(httpsUrl)}`;
}
