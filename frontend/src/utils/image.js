export function proxyImg(url) {
  if (!url) return '';
  // 已经是本地代理的不重复包装
  if (url.startsWith('http://127.0.0.1:54321/')) return url;
  // 非 http(s) 直接返回（base64 等）
  if (!url.startsWith('http')) return url;
  return `http://127.0.0.1:54321/img?u=${encodeURIComponent(url)}`;
}
