// WGCNA standard module colors -> hex, for rendering.
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
}

export function moduleColor(name: string): string {
  return MODULE_COLORS[name] ?? '#cccccc'
}
