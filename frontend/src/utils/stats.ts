// Statistical helpers shared by chart components.

export function pearson(a: number[], b: number[]): number {
  const n = a.length
  if (n < 2) return NaN
  let ma = 0
  let mb = 0
  for (let i = 0; i < n; i++) { ma += a[i]; mb += b[i] }
  ma /= n
  mb /= n
  let sxx = 0
  let syy = 0
  let sxy = 0
  for (let i = 0; i < n; i++) {
    const da = a[i] - ma
    const db = b[i] - mb
    sxx += da * da
    syy += db * db
    sxy += da * db
  }
  const den = Math.sqrt(sxx * syy)
  return den === 0 ? NaN : sxy / den
}

export function linearRegression(xs: number[], ys: number[]): { slope: number; intercept: number } {
  const n = xs.length
  if (n === 0) return { slope: 0, intercept: 0 }
  let mx = 0
  let my = 0
  for (let i = 0; i < n; i++) { mx += xs[i]; my += ys[i] }
  mx /= n
  my /= n
  let sxx = 0
  let sxy = 0
  for (let i = 0; i < n; i++) {
    const dx = xs[i] - mx
    sxx += dx * dx
    sxy += dx * (ys[i] - my)
  }
  const slope = sxx === 0 ? 0 : sxy / sxx
  return { slope, intercept: my - slope * mx }
}

// Two-sided Pearson correlation p-value, matching R's cor.test / WGCNA corPvalueStudent.
export function corPvalueStudent(r: number, n: number): number {
  if (n <= 2 || !Number.isFinite(r) || Math.abs(r) >= 1) {
    return Math.abs(r) >= 1 ? 0 : NaN
  }
  const df = n - 2
  const t = r * Math.sqrt(df / (1 - r * r))
  return 2 * studentTCdf(-Math.abs(t), df)
}

// Two-sided t critical value (e.g. tCritical(0.05, df) -> t_{0.975, df} for a
// 95% confidence interval). Bisection on the Student-t CDF.
export function tCritical(alpha: number, df: number): number {
  if (df < 1) return NaN
  const p = 1 - alpha / 2
  let lo = 0
  let hi = 100
  for (let i = 0; i < 100; i++) {
    const mid = (lo + hi) / 2
    if (studentTCdf(mid, df) < p) lo = mid
    else hi = mid
  }
  return (lo + hi) / 2
}

function studentTCdf(t: number, df: number): number {
  const x = df / (df + t * t)
  const ib = betai(df / 2, 0.5, x)
  return t < 0 ? 0.5 * ib : 1 - 0.5 * ib
}

function betai(a: number, b: number, x: number): number {
  if (x <= 0) return 0
  if (x >= 1) return 1
  const bt = Math.exp(lnGamma(a + b) - lnGamma(a) - lnGamma(b) + a * Math.log(x) + b * Math.log(1 - x))
  const fpmin = 1e-300
  const qab = a + b
  const qap = a + 1
  const qam = a - 1
  let c = 1
  let d = 1 - (qab * x) / qap
  if (Math.abs(d) < fpmin) d = fpmin
  d = 1 / d
  let h = d
  for (let m = 1; m <= 200; m++) {
    const m2 = 2 * m
    let aa = (m * (b - m) * x) / ((qam + m2) * (a + m2))
    d = 1 + aa * d
    if (Math.abs(d) < fpmin) d = fpmin
    c = 1 + aa / c
    if (Math.abs(c) < fpmin) c = fpmin
    d = 1 / d
    h *= d * c
    aa = (-(a + m) * (qab + m) * x) / ((a + m2) * (qap + m2))
    d = 1 + aa * d
    if (Math.abs(d) < fpmin) d = fpmin
    c = 1 + aa / c
    if (Math.abs(c) < fpmin) c = fpmin
    d = 1 / d
    const del = d * c
    h *= del
    if (Math.abs(del - 1) < 1e-15) break
  }
  return (bt * h) / a
}

function lnGamma(x: number): number {
  if (x < 0.5) {
    return Math.log(Math.PI / Math.sin(Math.PI * x)) - lnGamma(1 - x)
  }
  const cof = [
    76.18009172947146, -86.50532032941677, 24.01409824083091,
    -1.231739572450155, 0.1208650973866179e-2, -0.5395239384953e-5,
  ]
  const xx = x - 1
  let tmp = xx + 5.5
  tmp -= (xx + 0.5) * Math.log(tmp)
  let ser = 1.000000000190015
  for (let i = 0; i < cof.length; i++) ser += cof[i] / (xx + i + 1)
  return -tmp + Math.log(2.5066282746310005 * ser)
}
