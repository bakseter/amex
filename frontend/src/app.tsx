import { useState } from 'react';

import { useListInvoicesQuery } from '@/api/transaction';
import ExportBar from '@/components/export-bar';
import InvoiceRow from '@/components/invoice-row';
import Summary from '@/components/summary';
import TransactionTable from '@/components/transaction-table';
import UploadZone from '@/components/upload-zone';

const App = () => {
    const { data: invoices = [] } = useListInvoicesQuery();
    const [activeId, setActiveId] = useState<number | null>(null);

    // Auto-select the most recent invoice
    const selectedId = activeId ?? invoices[0]?.id ?? null;

    return (
        <div className="flex h-screen overflow-hidden font-sans text-gray-800 bg-white">
            {/* Left sidebar — invoice list */}
            <div className="w-64 shrink-0 border-r border-gray-100 flex flex-col overflow-hidden">
                <div className="px-4 py-3 border-b border-gray-100">
                    <div className="text-xs font-semibold uppercase tracking-widest text-gray-400 mb-2">
                        Invoices
                    </div>
                    <UploadZone onUploaded={setActiveId} />
                </div>
                <div className="flex-1 overflow-y-auto p-2 space-y-0.5">
                    {invoices.length === 0 && (
                        <div className="px-2 py-4 text-xs text-center text-gray-300">
                            No invoices yet
                        </div>
                    )}
                    {invoices.map((inv) => (
                        <InvoiceRow
                            key={inv.id}
                            invoice={inv}
                            active={inv.id === selectedId}
                            onSelect={() => {
                                setActiveId(inv.id);
                            }}
                        />
                    ))}
                </div>
            </div>

            {/* Main — transactions */}
            <div className="flex flex-col flex-1 overflow-hidden">
                {selectedId === null ? (
                    <div className="flex-1 flex items-center justify-center text-sm text-gray-300">
                        Upload an invoice to get started
                    </div>
                ) : (
                    <>
                        <TransactionTable invoiceId={selectedId} />
                        <ExportBar invoiceId={selectedId} />
                    </>
                )}
            </div>

            {/* Right sidebar — summary */}
            {selectedId !== null && (
                <div className="w-52 shrink-0 border-l border-gray-100 p-4 overflow-y-auto">
                    <Summary invoiceId={selectedId} />
                </div>
            )}
        </div>
    );
};

export default App;
