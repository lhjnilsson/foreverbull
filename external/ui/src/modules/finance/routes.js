const routes = [  {
  path: '',
  name: 'Finance',
  component: () => import('@/modules/finance/views/Finance.vue'),
},
]

export { routes }
