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
    const [showInvoices, setShowInvoices] = useState(false);

    // Auto-select the most recent invoice
    const selectedId = activeId ?? invoices[0]?.id;

    return (
        <div className="flex flex-col md:flex-row h-screen overflow-hidden font-sans text-gray-800 bg-white">
            {/* Left sidebar — invoice list */}
            <div
                className={`${showInvoices ? 'flex' : 'hidden'} md:flex w-full md:w-64 shrink-0 border-r border-gray-100 flex-col overflow-hidden absolute md:relative z-10 bg-white h-full`}
            >
                <div className="px-4 py-3 border-b border-gray-100">
                    <div className="text-xs font-semibold uppercase tracking-widest text-gray-400 mb-2">
                        Invoices
                    </div>
                    <UploadZone
                        onUploaded={(id) => {
                            setActiveId(id);
                            setShowInvoices(false);
                        }}
                    />
                </div>
                <div className="flex-1 overflow-y-auto p-2 space-y-0.5">
                    {invoices.length === 0 && (
                        <div className="px-2 py-4 text-xs text-center text-gray-300">
                            No invoices yet
                        </div>
                    )}
                    {invoices.map((invoice) => (
                        <InvoiceRow
                            key={invoice.id}
                            invoice={invoice}
                            active={invoice.id === selectedId}
                            onSelect={() => {
                                setActiveId(invoice.id);
                                setShowInvoices(false);
                            }}
                        />
                    ))}
                </div>
            </div>

            {/* Main — transactions */}
            <div className="flex flex-col flex-1 overflow-hidden">
                {selectedId ? (
                    <>
                        <div className="flex md:hidden items-center gap-2 px-3 py-2 border-b border-gray-100 text-xs">
                            <button
                                onClick={() => {
                                    setShowInvoices((show) => !show);
                                }}
                                className="text-blue-500 underline"
                            >
                                {showInvoices
                                    ? 'Hide invoices'
                                    : 'Switch invoice'}
                            </button>
                            <span className="text-gray-400 truncate ml-auto">
                                {
                                    invoices.find(
                                        (invoice) => invoice.id === selectedId
                                    )?.filename
                                }
                            </span>
                        </div>
                        <TransactionTable invoiceId={selectedId} />
                        <ExportBar invoiceId={selectedId} />
                    </>
                ) : (
                    <div className="flex-1 flex items-center justify-center text-sm text-gray-300">
                        Upload an invoice to get started
                    </div>
                )}
            </div>

            {/* Right sidebar — summary */}
            {selectedId && (
                <div className="hidden md:block w-52 shrink-0 border-l border-gray-100 p-4 overflow-y-auto">
                    <Summary invoiceId={selectedId} />
                </div>
            )}
        </div>
    );
};

export default App;
