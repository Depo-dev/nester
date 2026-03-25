
export type TransactionType = "Deposit" | "Withdrawal" | "Yield Accrual" | "Rebalance";
export type TransactionStatus = "Confirmed" | "Pending" | "Failed";

export interface Transaction {
    id: string;
    type: TransactionType;
    amount: string;
    asset: string;
    vaultName: string;
    timestamp: string;
    status: TransactionStatus;
    txHash: string;
}

const VAULTS = ["Conservative Yield", "Balanced Growth", "DeFi500 Index", "Growth Strategy"];
const ASSETS = ["USDC", "XLM"];

const generateMockTransactions = (count: number): Transaction[] => {
    const txs: Transaction[] = [];
    const now = new Date();
    
    for (let i = 0; i < count; i++) {
        const typeIdx = Math.floor(Math.random() * 4);
        const types: TransactionType[] = ["Deposit", "Withdrawal", "Yield Accrual", "Rebalance"];
        const type = types[typeIdx];
        
        const vault = VAULTS[Math.floor(Math.random() * VAULTS.length)];
        const asset = ASSETS[Math.floor(Math.random() * ASSETS.length)];
        
        const amount = type === "Rebalance" ? "0.00" : 
                       type === "Withdrawal" ? `-${(Math.random() * 500 + 50).toFixed(2)}` : 
                       `+${(Math.random() * 1500 + 10).toFixed(2)}`;
        
        const status: TransactionStatus = Math.random() > 0.9 ? "Failed" : (Math.random() > 0.8 ? "Pending" : "Confirmed");
        
        const date = new Date(now.getTime() - i * (Math.random() * 86400000 + 3600000));
        
        txs.push({
            id: (i + 1).toString(),
            type,
            amount,
            asset,
            vaultName: vault,
            timestamp: date.toISOString(),
            status,
            txHash: Array.from({ length: 64 }, () => Math.floor(Math.random() * 16).toString(16)).join(""),
        });
    }
    return txs;
};

export const mockTransactions: Transaction[] = generateMockTransactions(50);
