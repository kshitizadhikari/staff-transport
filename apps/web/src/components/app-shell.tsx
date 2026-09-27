"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import { RequireAuth } from "@/components/require-auth";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/lib/auth";
import { cn } from "@/lib/utils";

const MANAGER_NAV = [
  { href: "/", label: "Dashboard" },
  { href: "/trips", label: "Trips" },
  { href: "/staff", label: "Staff" },
  { href: "/drivers", label: "Drivers" },
  { href: "/vehicles", label: "Vehicles" },
];

function isActive(pathname: string, href: string) {
  return href === "/" ? pathname === "/" : pathname.startsWith(href);
}

export function AppShell({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();
  const pathname = usePathname();
  const nav = user?.role === "manager" ? MANAGER_NAV : MANAGER_NAV.slice(0, 1);

  return (
    <RequireAuth>
      <div className="flex min-h-screen flex-col">
        <header className="border-b">
          <div className="mx-auto flex w-full max-w-5xl flex-wrap items-center justify-between gap-3 px-6 py-3">
            <div className="flex items-center gap-6">
              <Link href="/" className="font-semibold tracking-tight">
                Staff Transport
              </Link>
              <nav className="flex items-center gap-1">
                {nav.map((item) => (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={cn(
                      "rounded-md px-2.5 py-1 text-sm transition-colors",
                      isActive(pathname, item.href)
                        ? "bg-muted font-medium text-foreground"
                        : "text-muted-foreground hover:text-foreground",
                    )}
                  >
                    {item.label}
                  </Link>
                ))}
              </nav>
            </div>
            <div className="flex items-center gap-3">
              <div className="hidden text-right sm:block">
                <p className="text-sm font-medium leading-tight">{user?.name}</p>
                <p className="text-xs capitalize text-muted-foreground">
                  {user?.role}
                </p>
              </div>
              <ThemeToggle />
              <Button variant="outline" size="sm" onClick={() => void logout()}>
                Sign out
              </Button>
            </div>
          </div>
        </header>
        <main className="mx-auto w-full max-w-5xl flex-1 px-6 py-8">
          {children}
        </main>
      </div>
    </RequireAuth>
  );
}
