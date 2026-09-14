// app.tsx
import { useState } from 'react';

import {
    useGetTransactionsQuery,
    useListInvoicesQuery,
} from '@/api/transaction';
import ExportBar from '@/components/export-bar';
import InvoiceRow from '@/components/invoice-row';
import Summary from '@/components/summary';
import TransactionTable from '@/components/transaction-table';
import UploadZone from '@/components/upload-zone';
import { monthLabel } from '@/utils/utils';

const App = () => {
    const { data: invoices = [] } = useListInvoicesQuery();
    const [activeId, setActiveId] = useState<number | null>(null);
    const [showInvoices, setShowInvoices] = useState(false);

    const selectedId = activeId ?? invoices[0]?.id;
    const selected = invoices.find((invoice) => invoice.id === selectedId);

    // Only for the header period label; the table fetches this too and RTK
    // Query dedupes the request.
    const { data: transactions = [] } = useGetTransactionsQuery(
        selectedId ?? 0,
        { skip: selectedId === undefined }
    );

    return (
        <div className="flex h-screen flex-col overflow-hidden bg-bg text-text md:flex-row">
            <aside
                className={`${showInvoices ? 'flex' : 'hidden'} absolute z-10 h-full w-full shrink-0 flex-col overflow-hidden border-r border-border bg-surface md:relative md:flex md:w-64`}
            >
                <div className="p-3">
                    <UploadZone
                        onUploaded={(id) => {
                            setActiveId(id);
                            setShowInvoices(false);
                        }}
                    />
                </div>
                <div className="flex-1 overflow-y-auto pb-2">
                    {invoices.length === 0 ? (
                        <div className="px-5 py-4 text-sm text-muted2">
                            No statements yet. Drop one above to get started.
                        </div>
                    ) : (
                        invoices.map((invoice) => (
                            <InvoiceRow
                                key={invoice.id}
                                invoice={invoice}
                                active={invoice.id === selectedId}
                                onSelect={() => {
                                    setActiveId(invoice.id);
                                    setShowInvoices(false);
                                }}
                            />
                        ))
                    )}
                </div>
            </aside>

            <main className="flex flex-1 flex-col overflow-hidden">
                {selectedId === undefined ? (
                    <div className="flex flex-1 items-center justify-center px-6 text-center text-muted2">
                        Upload an Amex CSV export to see who owes what.
                    </div>
                ) : (
                    <>
                        <header className="flex shrink-0 items-baseline gap-3 border-b border-border px-5 py-3">
                            <button
                                onClick={() => {
                                    setShowInvoices((show) => !show);
                                }}
                                className="text-sm text-muted2 hover:text-text md:hidden"
                            >
                                {showInvoices ? 'Close' : 'Statements'}
                            </button>
                            <h1 className="text-base font-semibold">
                                {monthLabel(transactions) || selected?.filename}
                            </h1>
                            <span className="truncate text-xs text-muted">
                                {selected?.filename}
                            </span>
                        </header>

                        <TransactionTable invoiceId={selectedId} />
                        <ExportBar invoiceId={selectedId} />
                    </>
                )}
            </main>

            {selectedId !== undefined && (
                <aside className="hidden w-64 shrink-0 overflow-y-auto border-l border-border p-5 lg:block">
                    <Summary invoiceId={selectedId} />
                </aside>
            )}
        </div>
    );
};

export default App;
