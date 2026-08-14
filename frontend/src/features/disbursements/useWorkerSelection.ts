import { useReducer } from "react";

import type { Worker } from "@/features/disbursements/queries";

type SelectionAction =
  | { type: "clear" }
  | { type: "remove"; workerIDs: ReadonlySet<string> }
  | { type: "replace"; workerIDs: readonly string[] }
  | { type: "toggle"; workerID: string }
  | { type: "toggle-all"; workerIDs: readonly string[] };

export function useWorkerSelection(workers: readonly Worker[]) {
  const [selectedWorkerIDs, dispatch] = useReducer(
    selectionReducer,
    undefined,
    () => new Set<string>(),
  );
  const selectedWorkers = workers.filter((worker) =>
    selectedWorkerIDs.has(worker.id),
  );

  return {
    clearSelection: () => dispatch({ type: "clear" }),
    removeWorkers: (workerIDs: ReadonlySet<string>) =>
      dispatch({ type: "remove", workerIDs }),
    replaceSelection: (workerIDs: readonly string[]) =>
      dispatch({ type: "replace", workerIDs }),
    selectedWorkerIDs,
    selectedWorkers,
    toggleAllWorkers: () =>
      dispatch({
        type: "toggle-all",
        workerIDs: workers.map((worker) => worker.id),
      }),
    toggleWorker: (workerID: string) => dispatch({ type: "toggle", workerID }),
  };
}

function selectionReducer(
  currentSelection: ReadonlySet<string>,
  action: SelectionAction,
): ReadonlySet<string> {
  switch (action.type) {
    case "clear":
      return new Set();
    case "replace":
      return new Set(action.workerIDs);
    case "remove":
      return new Set(
        [...currentSelection].filter(
          (workerID) => !action.workerIDs.has(workerID),
        ),
      );
    case "toggle": {
      const nextSelection = new Set(currentSelection);
      if (nextSelection.has(action.workerID)) {
        nextSelection.delete(action.workerID);
      } else {
        nextSelection.add(action.workerID);
      }
      return nextSelection;
    }
    case "toggle-all": {
      const allWorkersSelected = action.workerIDs.every((workerID) =>
        currentSelection.has(workerID),
      );
      return allWorkersSelected ? new Set() : new Set(action.workerIDs);
    }
  }
}
