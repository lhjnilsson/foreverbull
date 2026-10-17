// Protojson names. Image and OccurredAt keep the proto field spelling.

export type DateParts = {
  year?: number;
  month?: number;
  day?: number;
};

export type BacktestAsset = {
  symbol?: string;
  enabled?: boolean;
};

export type BacktestPeriod = {
  startDate?: DateParts;
  endDate?: DateParts;
  assets?: BacktestAsset[];
};

export type Backtest = {
  name?: string;
  benchmark?: string;
  periods?: BacktestPeriod[];
  provider?: string;
  profile?: string;
};

export type Strategy = {
  id?: string;
  name?: string;
  description?: string;
  createdAt?: string;
};

export type FunctionParameter = {
  key?: string;
  defaultValue?: string;
  valueType?: string;
};

export type AlgorithmFunction = {
  name?: string;
  parameters?: FunctionParameter[];
  parallelExecution?: boolean;
  runFirst?: boolean;
  runLast?: boolean;
};

export type Algorithm = {
  filePath?: string;
  functions?: AlgorithmFunction[];
  namespaces?: string[];
};

export type ServiceStatus = {
  status?: string;
  OccurredAt?: string;
};

export type Service = {
  Image?: string;
  algorithm?: Algorithm;
  statuses?: ServiceStatus[];
};
