export const MAX_SCREENSHOTS = 4;
export const MAX_REPLAYS = 3;
export const MAX_ATTACHMENT_BYTES = 9 * 1024 * 1024;
export const MAX_ATTACHMENT_MB = MAX_ATTACHMENT_BYTES / (1024 * 1024);
const MAX_DIMENSION = 2560;
const KEEP_AS_IS_BYTES = 1.5 * 1024 * 1024;
const JPEG_QUALITY = 0.85;

export function scaledSize(width, height, max = MAX_DIMENSION) {
  const scale = Math.min(1, max / Math.max(width, height));
  return { width: Math.round(width * scale), height: Math.round(height * scale) };
}

export function base64FromDataURL(dataURL) {
  return dataURL.slice(dataURL.indexOf(',') + 1);
}

export function isReplayFile(file) {
  return /\.rep$/i.test(file.name || '');
}

export function isImageFile(file) {
  return (file.type || '').startsWith('image/');
}

export function attachmentBytes(screenshots, replays) {
  return [...screenshots, ...replays].reduce((sum, item) => sum + item.size, 0);
}

const readDataURL = (blob) => new Promise((resolve, reject) => {
  const reader = new FileReader();
  reader.onload = () => resolve(reader.result);
  reader.onerror = () => reject(reader.error);
  reader.readAsDataURL(blob);
});

// The Worker only accepts PNG and JPEG under a total size cap, so anything else
// (or anything big) is re-encoded as a downscaled JPEG.
async function normalized(file) {
  const keepAsIs = (file.type === 'image/png' || file.type === 'image/jpeg') && file.size <= KEEP_AS_IS_BYTES;
  if (keepAsIs) return file;
  const bitmap = await createImageBitmap(file);
  const { width, height } = scaledSize(bitmap.width, bitmap.height);
  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  canvas.getContext('2d').drawImage(bitmap, 0, 0, width, height);
  bitmap.close?.();
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => (blob ? resolve(blob) : reject(new Error('encode failed'))), 'image/jpeg', JPEG_QUALITY);
  });
}

export async function prepareScreenshot(file) {
  const blob = await normalized(file);
  return {
    base64: base64FromDataURL(await readDataURL(blob)),
    preview: URL.createObjectURL(blob),
    size: blob.size,
  };
}

export async function prepareReplay(file) {
  return {
    name: file.name,
    base64: base64FromDataURL(await readDataURL(file)),
    size: file.size,
  };
}
