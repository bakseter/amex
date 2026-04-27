import { useMemo, useState } from 'react';

import {
    type Transaction,
    useGetMetaQuery,
    useGetSummaryQuery,
    useGetTransactionsQuery,
} from '@/api/transaction';
import Header from '@/components/header';
import Sidebar from '@/components/sidebar';
import Toolbar from '@/components/toolbar';
import TransactionTable from '@/components/transaction-table';

const App = () => {
    const { data: transactions, isLoading: transactionsAreLoading } =
        useGetTransactionsQuery();
    const { data: meta, isLoading: metaIsLoading } = useGetMetaQuery();
    const { data: summary, isLoading: summaryIsLoading } = useGetSummaryQuery();

    const isLoading =
        transactionsAreLoading || metaIsLoading || summaryIsLoading;

    const [selectedId, setSelectedId] = useState(null);
    const [filters, setFilters] = useState({
        query: '',
        holder: '',
        category: '',
        uncategorized: false,
    });

    // Filtered list
    const visible = useMemo(() => {
        const { query, holder, category, uncategorized } = filters;

        return (
            transactions?.filter((transaction: Transaction) => {
                if (
                    query &&
                    !transaction.description
                        .toLowerCase()
                        .includes(query.toLowerCase()) &&
                    !transaction.category
                        .toLowerCase()
                        .includes(query.toLowerCase())
                ) {
                    return false;
                }
                if (holder && transaction.cardholder !== holder) {
                    return false;
                }
                if (category && transaction.category !== category) {
                    return false;
                }
                if (uncategorized && transaction.category !== 'Uncategorized') {
                    return false;
                }

                return true;
            }) ?? []
        );
    }, [transactions, filters]);

    if (isLoading) {
        return (
            <div className="flex items-center justify-center h-full text-(--color-muted) text-xs tracking-widest uppercase">
                Loading…
            </div>
        );
    }

    return (
        <div className="flex flex-col h-screen overflow-hidden">
            <Header meta={meta} transactions={transactions} />

            <div className="flex flex-1 overflow-hidden">
                {/* Left: table */}
                <div className="flex flex-col flex-1 overflow-hidden">
                    <Toolbar
                        filters={filters}
                        setFilters={setFilters}
                        meta={meta}
                        visibleCount={visible.length}
                    />
                    <TransactionTable
                        transactions={visible}
                        selectedId={selectedId}
                        onSelect={setSelectedId}
                        meta={meta}
                    />
                </div>

                {/* Right: sidebar */}
                <div className="w-72 shrink-0">
                    <Sidebar
                        selectedId={selectedId}
                        transactions={transactions}
                        summary={summary}
                        meta={meta}
                    />
                </div>
            </div>
        </div>
    );
};

export default App;
