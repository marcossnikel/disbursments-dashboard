import ShieldCheck from "lucide-react/dist/esm/icons/shield-check.mjs";

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
        <section className="overflow-hidden rounded-2xl bg-secondary px-6 py-9 text-center text-secondary-foreground shadow-xl shadow-black/10 sm:px-10 sm:py-11">
          <Badge className="mb-4 rounded-full bg-primary/15 px-3 py-1 text-[#ff7047] hover:bg-primary/15">
            Payment operations
          </Badge>
          <h1 className="text-4xl leading-none font-semibold tracking-[-0.04em] sm:text-5xl">
            Payroll disbursements
          </h1>
          <p className="mx-auto mt-4 max-w-2xl text-base leading-7 text-white/65">
            Review pending worker payments, confirm a batch, and monitor each
            provider result.
          </p>
        </section>

        <section className="mt-6" aria-label="Disbursements">
          <DisbursementDashboard />
        </section>
      </main>
    </div>
  );
}
