// Count Unicode code points (not UTF-16 code units), matching the server's
// utf8.RuneCountInString so client-side length validation agrees with the Go
// boundary rules. Plain String.length over-counts non-BMP input (e.g. emoji),
// which would reject values the server accepts.
export function runeLength(value: string): number {
  return [...value].length
}
