"use client";

import { useEffect, useState } from "react";

import { api } from "@/lib/api";

// Current user's permissions from /api/rbac/me control navigation/button visibility.
// Module-level cache fetches once per session.
let cached: Set<string> | null = null;
let inflight: Promise<Set<string>> | null = null;

async function fetchPerms(): Promise<Set<string>> {
  if (cached) return cached;
  if (!inflight) {
    inflight = api
      .rbacMe()
      .then((me) => {
        cached = new Set(me.permissions ?? []);
        return cached;
      })
      .catch(() => new Set<string>())
      .finally(() => {
        inflight = null;
      });
  }
  return inflight;
}

export function usePermissions() {
  const [perms, setPerms] = useState<Set<string>>(cached ?? new Set());
  const [ready, setReady] = useState(cached !== null);

  useEffect(() => {
    let alive = true;
    void fetchPerms().then((p) => {
      if (alive) {
        setPerms(p);
        setReady(true);
      }
    });
    return () => {
      alive = false;
    };
  }, []);

  return {
    ready,
    permissions: perms,
    has: (p: string) => perms.has(p),
  };
}
