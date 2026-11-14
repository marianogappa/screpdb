import { withDpi } from './pngDpi.js';

const readAsDataURL = (blob) => new Promise((resolve, reject) => {
  const reader = new FileReader();
  reader.onload = () => resolve(reader.result);
  reader.onerror = () => reject(reader.error);
  reader.readAsDataURL(blob);
});

// The PNG is drawn like a screenshot on a high density screen: twice the pixels, on whole pixels so lines stay
// sharp, and marked at twice the usual 72 dpi so viewers that read it show it at the card's size.
const PNG_SCALE = 2;

// svgToPng draws an on-page SVG as a PNG in the browser. An SVG drawn as an image loads nothing itself, so its
// images are inlined first.
export async function svgToPng(svg, scale = PNG_SCALE) {
  const { width, height } = svg.viewBox.baseVal;
  const clone = svg.cloneNode(true);
  clone.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
  clone.setAttribute('width', String(width * scale));
  clone.setAttribute('height', String(height * scale));
  await Promise.all([...clone.querySelectorAll('image')].map(async (image) => {
    const href = image.getAttribute('href');
    if (!href || href.startsWith('data:')) return;
    const response = await fetch(href);
    image.setAttribute('href', await readAsDataURL(await response.blob()));
  }));
  const svgURL = URL.createObjectURL(new Blob([new XMLSerializer().serializeToString(clone)], { type: 'image/svg+xml' }));
  try {
    const img = new Image();
    img.src = svgURL;
    await img.decode();
    const canvas = document.createElement('canvas');
    canvas.width = width * scale;
    canvas.height = height * scale;
    const context = canvas.getContext('2d');
    context.imageSmoothingQuality = 'high';
    context.drawImage(img, 0, 0);
    const png = await new Promise((resolve) => canvas.toBlob(resolve, 'image/png'));
    return new Blob([withDpi(new Uint8Array(await png.arrayBuffer()), 72 * scale)], { type: 'image/png' });
  } finally {
    URL.revokeObjectURL(svgURL);
  }
}

// downloadSvgAsPng saves an on-page SVG as a PNG named fileName.
export async function downloadSvgAsPng(svg, fileName) {
  const link = document.createElement('a');
  link.href = URL.createObjectURL(await svgToPng(svg));
  link.download = fileName;
  link.click();
  setTimeout(() => URL.revokeObjectURL(link.href), 1000);
}
