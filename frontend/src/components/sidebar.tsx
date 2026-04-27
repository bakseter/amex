import { type Meta, type Summary, type Transaction } from '@/api/transaction';
import ContextPanel from '@/components/context-panel';
import DetailPanel from '@/components/detail-panel';
import ExportButton from '@/components/export-button';
import SummaryPanel from '@/components/summary-panel';

interface Props {
    selectedId: string | null;
    transactions: Transaction[];
    summary: Summary;
    meta: Meta;
    onPatch: (id: string, patch: Partial<Transaction>) => void;
    onExport: () => Promise<{ transactionPath: string }>;
    exportStatus: { isPending: boolean };
}

const Sidebar = ({
    selectedId,
    transactions,
    summary,
    meta,
    onPatch,
    onExport,
    exportStatus,
}: Props) => {
    const selected =
        transactions.find(
            (transaction) => String(transaction.id) === selectedId
        ) ?? null;
    const idx = transactions.findIndex(
        (transaction) => String(transaction.id) === selectedId
    );
    const nearby =
        idx >= 0
            ? transactions.slice(
                  Math.max(0, idx - 4), // eslint-disable-line no-magic-numbers
                  Math.min(transactions.length, idx + 5) // eslint-disable-line no-magic-numbers
              )
            : [];

    return (
        <aside className="border-l border-(--color-border) flex flex-col overflow-hidden bg-(--color-surface)">
            {/* Summary */}
            <SummaryPanel summary={summary} />

            {/* Detail + context, scrollable */}
            <div className="flex-1 overflow-y-auto">
                {selected ? (
                    <>
                        <DetailPanel
                            transaction={selected}
                            meta={meta}
                            onPatch={onPatch}
                        />

                        <ContextPanel nearby={nearby} selectedId={selectedId} />
                    </>
                ) : (
                    <div className="p-4 text-(--color-muted) text-[11px] italic">
                        ← select a transaction to review
                    </div>
                )}
            </div>

            <ExportButton onExport={onExport} status={exportStatus} />
        </aside>
    );
};

export default Sidebar;
