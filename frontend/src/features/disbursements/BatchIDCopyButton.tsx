import { AnimatePresence, motion } from "motion/react";
import Check from "lucide-react/dist/esm/icons/check.mjs";
import Copy from "lucide-react/dist/esm/icons/copy.mjs";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";

type BatchIDCopyButtonProps = {
  batchID: string;
};

export function BatchIDCopyButton({ batchID }: BatchIDCopyButtonProps) {
  const [isCopied, setCopied] = useState(false);
  const resetTimer = useRef<number | undefined>(undefined);

  useEffect(
    () => () => {
      if (resetTimer.current !== undefined) {
        window.clearTimeout(resetTimer.current);
      }
    },
    [],
  );

  async function copyBatchID() {
    await navigator.clipboard.writeText(batchID);
    setCopied(true);
    if (resetTimer.current !== undefined) {
      window.clearTimeout(resetTimer.current);
    }
    resetTimer.current = window.setTimeout(() => setCopied(false), 1_800);
  }

  return (
    <Button
      variant="outline"
      size="sm"
      className="h-8 gap-2 rounded-lg bg-white px-2 font-mono text-xs text-muted-foreground hover:text-foreground"
      onClick={() => void copyBatchID()}
      aria-label={isCopied ? "Batch ID copied" : "Copy batch ID"}
    >
      <span className="max-w-48 truncate">{batchID}</span>
      <AnimatePresence mode="wait" initial={false}>
        <motion.span
          key={isCopied ? "copied" : "copy"}
          initial={{ opacity: 0, scale: 0.6, rotate: -12 }}
          animate={{ opacity: 1, scale: 1, rotate: 0 }}
          exit={{ opacity: 0, scale: 0.6, rotate: 12 }}
          transition={{ duration: 0.15 }}
        >
          {isCopied ? (
            <Check
              aria-hidden="true"
              className="size-3.5 text-status-success"
            />
          ) : (
            <Copy aria-hidden="true" className="size-3.5" />
          )}
        </motion.span>
      </AnimatePresence>
    </Button>
  );
}
