import { useCallback, useState } from "react";

export function useSearchState<T>() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<T[]>([]);
  const [searching, setSearching] = useState(false);
  const [hasSearched, setHasSearched] = useState(false);

  const clear = useCallback(() => {
    setQuery("");
    setResults([]);
    setHasSearched(false);
  }, []);

  const run = useCallback(async (search: (query: string) => Promise<T[]>) => {
    const nextQuery = query.trim();
    if (!nextQuery || searching) return;
    setSearching(true);
    setHasSearched(true);
    try {
      setResults(await search(nextQuery));
    } finally {
      setSearching(false);
    }
  }, [query, searching]);

  return { query, setQuery, results, setResults, searching, setSearching, hasSearched, setHasSearched, clear, run };
}
