import { useRef } from 'react';

export function useServerRequest() {
  const generation = useRef(0);
  return {
    next() {
      generation.current += 1;
      return generation.current;
    },
    isCurrent(ticket: number) {
      return ticket === generation.current;
    },
  };
}
