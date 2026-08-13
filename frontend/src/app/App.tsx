import { ShieldCheck } from "lucide-react";

import cadanaLogo from "@/assets/cadana-logo.jpeg";
import { Badge } from "@/components/ui/badge";
import { DisbursementDashboard } from "@/features/disbursements/DisbursementDashboard";

export function App() {
  return (
    <div className="min-h-screen">
      <header className="border-b border-black/5 bg-white/90 backdrop-blur">
        <div className="mx-auto flex min-h-18 max-w-7xl items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
          <div className="flex items-center gap-3">
            <img
              src={cadanaLogo}
              alt="Cadana"
              className="size-10 rounded-xl border border-black/5 object-cover shadow-sm"
            />
            <div>
              <p className="text-sm font-semibold tracking-tight">Cadana</p>
              <p className="text-xs text-muted-foreground">
                Disbursement Console
              </p>
            </div>
          </div>
          <Badge
            variant="outline"
            className="gap-1.5 rounded-full bg-white px-3 py-1"
          >
            <ShieldCheck
              aria-hidden="true"
              className="size-3.5 text-status-success"
            />
            Internal operations
          </Badge>
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 sm:py-10 lg:px-8">
        <section className="overflow-hidden rounded-3xl bg-secondary px-6 py-10 text-secondary-foreground shadow-2xl shadow-black/10 sm:px-10 sm:py-14">
          <Badge className="mb-5 rounded-full bg-primary/15 px-3 py-1 text-primary hover:bg-primary/15">
            Payment operations
          </Badge>
          <h1 className="max-w-3xl text-4xl leading-[0.98] font-semibold tracking-[-0.05em] sm:text-6xl">
            Payroll disbursements,
            <span className="font-serif font-normal italic text-primary">
              {" "}
              made clear.
            </span>
          </h1>
          <p className="mt-5 max-w-xl text-base leading-7 text-white/65">
            Select pending workers, confirm one immutable batch, and follow
            every provider result as it arrives.
          </p>
        </section>

        <section
          className="relative z-10 -mt-3 px-0 sm:-mt-8 sm:px-6"
          aria-label="Disbursements"
        >
          <DisbursementDashboard />
        </section>
      </main>
    </div>
  );
}
