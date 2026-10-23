// WGCNA standard module colors -> hex, for rendering.
// Matches rs_WGCNA's STANDARD_COLORS (50 colors) so every module has a color.
export const MODULE_COLORS: Record<string, string> = {
  turquoise: '#1f9e89',
  blue: '#3182bd',
  brown: '#8c510a',
  yellow: '#e6ab02',
  green: '#66a61e',
  red: '#e41a1c',
  black: '#252525',
  pink: '#f781bf',
  magenta: '#984ea3',
  purple: '#6a51a3',
  greenyellow: '#a6d854',
  tan: '#b89b74',
  salmon: '#fa8072',
  cyan: '#00ced1',
  midnightblue: '#191970',
  lightcyan: '#e0ffff',
  grey: '#999999',
  grey60: '#999999',
  lightgreen: '#90ee90',
  lightyellow: '#ffffe0',
  royalblue: '#4169e1',
  darkred: '#8b0000',
  darkgreen: '#006400',
  darkturquoise: '#00ced1',
  darkgrey: '#a9a9a9',
  orange: '#ffa500',
  darkorange: '#ff8c00',
  white: '#ffffff',
  skyblue: '#87ceeb',
  saddlebrown: '#8b4513',
  steelblue: '#4682b4',
  paleturquoise: '#afeeee',
  violet: '#ee82ee',
  darkolivegreen: '#556b2f',
  darkmagenta: '#8b008b',
  sienna3: '#cd5c5c',
  yellowgreen: '#9acd32',
  skyblue3: '#6ca6cd',
  plum1: '#ffbbff',
  orangered4: '#801700',
  mediumpurple3: '#8968cd',
  lightsteelblue1: '#cae1ff',
  lightcyan1: '#e0ffff',
  ivory: '#fffff0',
  floralwhite: '#fffaf0',
  darkorange2: '#ee7600',
  brown4: '#8b2323',
  bisque4: '#8b7d6b',
  darkslateblue: '#483d8b',
  plum2: '#eeaeee',
  thistle2: '#eed2ee',
}

export function moduleColor(name: string): string {
  return MODULE_COLORS[name] ?? '#cccccc'
}

function parseHex(hex: string): { r: number; g: number; b: number } {
  let h = hex.replace('#', '')
  if (h.length === 3) h = h.split('').map((c) => c + c).join('')
  const n = parseInt(h, 16)
  if (Number.isNaN(n)) return { r: 204, g: 204, b: 204 }
  return { r: (n >> 16) & 255, g: (n >> 8) & 255, b: n & 255 }
}

// Linear interpolation between two hex colors. t is clamped to [0, 1].
export function mixHex(a: string, b: string, t: number): string {
  const pa = parseHex(a)
  const pb = parseHex(b)
  const c = Math.max(0, Math.min(1, t))
  const r = Math.round(pa.r + (pb.r - pa.r) * c)
  const g = Math.round(pa.g + (pb.g - pa.g) * c)
  const bl = Math.round(pa.b + (pb.b - pa.b) * c)
  return `rgb(${r},${g},${bl})`
}
