// A faithful port of Go's math/rand/v2 PCG source plus the Rand helpers the
// simulator uses (Float64, IntN, Uint32, Uint64). Reproducing the generator
// bit for bit is what lets the browser produce the exact same incident bundle
// as the Go collector for a given seed and scenario.
//
// Go seeds the source with rand.NewPCG(uint64(seed), 0x9e3779b97f4a7c15) and
// draws values through rand.New(source). All arithmetic is 64-bit and done with
// BigInt so wraparound matches Go's uint64 semantics exactly.

const MASK64 = (1n << 64n) - 1n;
const MASK32 = (1n << 32n) - 1n;

const MUL_HI = 2549297995355413924n;
const MUL_LO = 4865540595714422341n;
const INC_HI = 6364136223846793005n;
const INC_LO = 1442695040888963407n;

// PCG is the 128-bit state PCG-XSL-RR generator from Go's math/rand/v2.
export class PCG {
  private hi: bigint;
  private lo: bigint;

  constructor(seed1: bigint, seed2: bigint) {
    this.hi = seed1 & MASK64;
    this.lo = seed2 & MASK64;
  }

  private mul64(x: bigint, y: bigint): [bigint, bigint] {
    const full = (x & MASK64) * (y & MASK64);
    return [(full >> 64n) & MASK64, full & MASK64];
  }

  private add64(xHi: bigint, xLo: bigint, yHi: bigint, yLo: bigint): [bigint, bigint] {
    const lo = (xLo + yLo) & MASK64;
    let hi = (xHi + yHi) & MASK64;
    if (lo < xLo) hi = (hi + 1n) & MASK64;
    return [hi, lo];
  }

  private next(): [bigint, bigint] {
    // state = state * multiplier + increment (128-bit)
    let [hi, lo] = this.mul64(this.lo, MUL_LO);
    hi = (hi + this.hi * MUL_LO + this.lo * MUL_HI) & MASK64;
    [hi, lo] = this.add64(hi, lo, INC_HI, INC_LO);
    this.lo = lo;
    this.hi = hi;
    return [hi, lo];
  }

  // Uint64 mirrors PCG.Uint64 from Go's math/rand/v2/pcg.go, which applies a
  // custom output mixing function to the 128-bit post-advance state rather than
  // the textbook XSL-RR permutation.
  uint64(): bigint {
    let [hi, lo] = this.next();
    const cheapMul = 0xda942042e4dd58b5n;
    hi = hi ^ (hi >> 32n);
    hi = (hi * cheapMul) & MASK64;
    hi = hi ^ (hi >> 48n);
    hi = (hi * ((lo | 1n) & MASK64)) & MASK64;
    return hi & MASK64;
  }
}

// Rand wraps a PCG source and exposes the same helpers the Go simulator calls.
export class Rand {
  private src: PCG;

  constructor(seed: bigint) {
    this.src = new PCG(seed & MASK64, 0x9e3779b97f4a7c15n);
  }

  uint64(): bigint {
    return this.src.uint64();
  }

  uint32(): number {
    // Go's Uint32 returns the top 32 bits of a Uint64 draw.
    return Number((this.src.uint64() >> 32n) & MASK32) >>> 0;
  }

  // Float64 matches rand/v2: float64(Uint64()<<11>>11) / (1<<53), i.e. the low
  // 53 bits of a Uint64 draw scaled into [0, 1).
  float64(): number {
    const bits = this.src.uint64() & ((1n << 53n) - 1n);
    return Number(bits) / 9007199254740992;
  }

  // intN matches rand/v2 uint64n for a signed int argument: rejection-free
  // Lemire multiply-shift with a bias fixup loop.
  intN(n: number): number {
    if (n <= 0) throw new Error("invalid argument to intN");
    return Number(this.uint64n(BigInt(n)));
  }

  private uint64n(n: bigint): bigint {
    // Lemire's method as implemented in Go's math/rand/v2 uint64n.
    let hi = this.mulHi(this.src.uint64(), n);
    let lo = this.lastMulLo;
    if (lo < n) {
      const thresh = (-n & MASK64) % n;
      while (lo < thresh) {
        hi = this.mulHi(this.src.uint64(), n);
        lo = this.lastMulLo;
      }
    }
    return hi;
  }

  private lastMulLo = 0n;
  private mulHi(x: bigint, y: bigint): bigint {
    const full = (x & MASK64) * (y & MASK64);
    this.lastMulLo = full & MASK64;
    return (full >> 64n) & MASK64;
  }
}
