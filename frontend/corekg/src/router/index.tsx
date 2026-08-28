import { lazy } from 'react'
import { Outlet, createBrowserRouter } from 'react-router-dom'
import AppLayout from '@/pages/app/layout'
import EmbedLayout from '@/pages/embed/layout'
import { LoginGlobalProvider } from '@/utils/useLoginGlobalData'
import { withSuspense } from '@/utils/withSuspense'
import AuthRouter from './AuthRouter'
import { DocsAuth } from './DocsAuth'
import ErrorPage from './ErrorPage'

const CADDetailView = withSuspense(
  lazy(() => import('@/pages/app/docs/cad/components/FileDetailView')),
)
const CADBaseDetail = withSuspense(
  lazy(() => import('@/pages/app/docs/cad/index')),
)
const DetailPromptAgent = withSuspense(
  lazy(() => import('@/pages/app/agents/detail/prompt/index')),
)
const Excel = withSuspense(lazy(() => import('@/pages/app/docs/excel/index')))
const DB = withSuspense(lazy(() => import('@/pages/app/docs/db/index')))
const DetailQuestionAgent = withSuspense(
  lazy(() => import('@/pages/app/agents/detail/question/index')),
)
const DetailRoleAgent = withSuspense(
  lazy(() => import('@/pages/app/agents/detail/role/index')),
)

const DetailPromptEmbed = withSuspense(
  lazy(() => import('@/pages/embed/detail/prompt/index')),
)
const DetailQuestionEmbed = withSuspense(
  lazy(() => import('@/pages/embed/detail/question/index')),
)
const DetailRoleEmbed = withSuspense(
  lazy(() => import('@/pages/embed/detail/role/index')),
)

const EmbedWidget = withSuspense(
  lazy(() => import('@/pages/embed/widget/index.tsx')),
)

const AgentSettings = withSuspense(
  lazy(() => import('@/pages/app/agents/settings')),
)
const AgentEdit = withSuspense(lazy(() => import('@/pages/app/agents/edit')))
const AgentAccess = withSuspense(
  lazy(() => import('@/pages/app/agents/access')),
)
const Agents = withSuspense(
  lazy(() => import('@/pages/app/agents/index/index')),
)
const WordCloudPage = withSuspense(
  lazy(() => import('@/pages/app/docs/detail/WordCloudPage')),
)
const FileDetailView = withSuspense(
  lazy(() => import('@/pages/app/docs/detail/components/FileDetailView')),
)
const FileEditView = withSuspense(
  lazy(() => import('@/pages/app/docs/detail/components/FileEditView')),
)
const KnowledgeBaseDetail = withSuspense(
  lazy(() => import('@/pages/app/docs/detail/index')),
)

const Docs = withSuspense(lazy(() => import('@/pages/app/docs/index')))
const QALibraryDetail = withSuspense(
  lazy(() => import('@/pages/app/docs/qa/index')),
)
const QA = withSuspense(lazy(() => import('@/pages/app/home/QA')))
const Home = withSuspense(lazy(() => import('@/pages/app/home/index')))
const GlobalHome = withSuspense(
  lazy(() => import('@/pages/app/home/GlobalHome')),
)
const Search = withSuspense(lazy(() => import('@/pages/app/home/search')))
const Studios = withSuspense(lazy(() => import('@/pages/app/studios/index')))
const ApplicationList = withSuspense(
  lazy(() => import('@/pages/app/applications/index')),
)
const ApplicationDetail = withSuspense(
  lazy(() => import('@/pages/app/applications/detail')),
)
const ApplicationCreate = withSuspense(
  lazy(() => import('@/pages/app/applications/create')),
)
const Announcement = withSuspense(
  lazy(() => import('@/pages/app/announcement')),
)
const Invite = withSuspense(lazy(() => import('@/pages/invite')))
const Callback = withSuspense(
  lazy(() => import('@/pages/other/callback/index')),
)
const Login = withSuspense(lazy(() => import('@/pages/other/login/index')))
const CLIAuth = withSuspense(lazy(() => import('@/pages/other/cli-authorize/index')))
const Register = withSuspense(
  lazy(() => import('@/pages/other/register/index')),
)
const Test = withSuspense(lazy(() => import('@/pages/other/test')))
const ProfileApiKey = withSuspense(
  lazy(() => import('@/pages/profile/api-key/index')),
)
const ProfileLayout = withSuspense(lazy(() => import('@/pages/profile/layout')))
const ChangeWx = withSuspense(
  lazy(() => import('@/pages/profile/my-info/components/ChangeWx')),
)
const ProfileMyInfo = withSuspense(
  lazy(() => import('@/pages/profile/my-info/index')),
)
const ProfilePurchase = withSuspense(
  lazy(() => import('@/pages/profile/purchase/index')),
)
const ProfileUsage = withSuspense(
  lazy(() => import('@/pages/profile/usage/index')),
)
const UserManagement = withSuspense(lazy(() => import('@/pages/settings')))
const SettingLayout = withSuspense(
  lazy(() => import('@/pages/settings/layout')),
)
const Version = withSuspense(lazy(() => import('@/pages/version')))
const ModelSettings = withSuspense(lazy(() => import('@/pages/settings/model')))
const OrganizationManagement = withSuspense(
  lazy(() => import('@/pages/organization')),
)
const PersonnelDirectory = withSuspense(lazy(() => import('@/pages/personnel')))
const OrderManagement = withSuspense(
  lazy(() => import('@/pages/settings/order-management')),
)
const MyLikes = withSuspense(lazy(() => import('@/pages/settings/MyLikes')))
const MyCollections = withSuspense(
  lazy(() => import('@/pages/settings/MyCollections')),
)

const Graph = withSuspense(lazy(() => import('@/pages/graph')))
const GraphDetail = withSuspense(lazy(() => import('@/pages/graph/detail')))
const GraphEdit = withSuspense(lazy(() => import('@/pages/graph/edit')))
const GraphSearchRelationship = withSuspense(
  lazy(() => import('@/pages/graph/search-relationship')),
)

const Auth = withSuspense(lazy(() => import('@/pages/auth')))
const Global = withSuspense(lazy(() => import('@/pages/global')))
const Project = withSuspense(lazy(() => import('@/pages/project')))

const AccountBindings = withSuspense(
  lazy(() => import('@/pages/settings/AccountBindings')),
)

const DotpenWeb = withSuspense(
  lazy(() => import('@/pages/other/dotpen-web/index')),
)

const TagGroupManagement = withSuspense(
  lazy(() => import('@/pages/settings/TagGroup')),
)
const TagManagement = withSuspense(lazy(() => import('@/pages/settings/Tag')))
const SynonymManagement = withSuspense(
  lazy(() => import('@/pages/settings/Synonym')),
)
const IndustryTermManagement = withSuspense(
  lazy(() => import('@/pages/settings/IndustryTerm')),
)

const AgentResources = withSuspense(
  lazy(() => import('@/pages/app/agents/resources')),
)

const Personnel = withSuspense(lazy(() => import('@/pages/personnel')))
const router = createBrowserRouter(
  [
    // license页面需要登录
    {
      path: '/auth',
      element: (
        <AuthRouter>
          <Auth />
        </AuthRouter>
      ),
    },
    { path: '/global-pilot', element: <Global /> },
    { path: '/login', element: <Login /> },
    { path: '/cli/authorize', element: <CLIAuth /> },
    { path: '/register', element: <Register /> },
    { path: '/callback', element: <Callback /> },
    { path: '/test', element: <Test /> },
    // 跨项目账号体系打通 - 从外部项目跳转入口
    { path: '/dotpen-web', element: <DotpenWeb /> },
    { path: '/changeWx', element: <ChangeWx /> },
    {
      path: '/',
      element: (
        <AuthRouter>
          <LoginGlobalProvider>
            <Outlet />
          </LoginGlobalProvider>
        </AuthRouter>
      ),
      errorElement: <ErrorPage />,
      children: [
        {
          path: '',
          element: <AppLayout />,
          errorElement: <ErrorPage />,
          children: [
            // { index: true, element: <Home /> },
            { path: 'global', element: <GlobalHome /> },
            { path: 'search', element: <Search /> },
            { path: 'QA', element: <QA /> },
            { path: 'announcement/:id?', element: <Announcement /> },
            // {
            //   path: 'agents/:id',
            //   element: <AgentSettings />,
            //   children: [
            //     { index: true, element: <Navigate to='edit' replace /> }, // 默认重定向到应用配置
            //     { path: 'edit', element: <AgentEdit /> },
            //     { path: 'access', element: <AgentAccess /> },
            //   ],
            // },
            { path: 'agents', element: <AgentResources /> },
            { path: 'agents/resources', element: <AgentResources /> },
            { path: 'agents/edit/:id', element: <AgentEdit /> },
            { path: 'agents/detail/role/:id', element: <DetailRoleAgent /> },
            {
              path: 'agents/detail/prompt/:id',
              element: <DetailPromptAgent />,
            },
            {
              // 工作流和指令型的用法一样
              path: 'agents/detail/workflow/:id',
              element: <DetailPromptAgent workflow />,
            },
            {
              path: 'agents/detail/question/:id',
              element: <DetailQuestionAgent />,
            },
            {
              path: 'docs',
              index: true,
              element: <Docs />,
            },
            {
              path: 'docs',
              element: (
                <DocsAuth>
                  <Outlet />
                </DocsAuth>
              ),
              children: [
                {
                  path: 'detail/:id',
                  element: <KnowledgeBaseDetail />,
                },
                {
                  path: 'detail/:id/folder/:folderId',
                  element: <KnowledgeBaseDetail />,
                },
                {
                  path: 'detail/:id/file/:fileId',
                  element: <FileDetailView />,
                },

                {
                  path: 'detail/:id/file/:fileId/edit',
                  element: <FileEditView />,
                },
                {
                  path: 'excel/:id',
                  element: <Excel />,
                },
                {
                  path: 'excel/:id/folder/:folderId',
                  element: <Excel />,
                },
                {
                  path: 'db/:id/*',
                  element: <DB />,
                },
                {
                  path: 'cad/:id',
                  element: <CADBaseDetail />,
                },
                {
                  path: 'cad/:id/folder/:folderId',
                  element: <CADBaseDetail />,
                },
                {
                  path: 'cad/:id/file/:fileId',
                  element: <CADDetailView />,
                },
                {
                  path: 'qa/:id',
                  element: <QALibraryDetail />,
                },
                {
                  path: ':id/wordcloud',
                  element: <WordCloudPage />,
                },
                {
                  path: ':id/knowledge-graph',
                  element: <WordCloudPage />,
                },
              ],
            },
            { path: 'studios', element: <Studios /> },
            { path: 'apps', element: <ApplicationList /> },
            { path: 'apps/create', element: <ApplicationCreate /> },
            { path: 'apps/:id', element: <ApplicationDetail /> },
            {
              path: 'graph',
              children: [
                {
                  index: true,
                  element: <Graph />,
                },
                {
                  children: [
                    {
                      path: 'detail',
                      element: <GraphDetail />,
                    },
                    {
                      path: 'edit',
                      element: <GraphEdit />,
                    },
                    {
                      path: 'search-relationship',
                      element: <GraphSearchRelationship />,
                    },
                  ],
                },
              ],
            },
            {
              path: 'personnel',
              element: <PersonnelDirectory />,
            },
            {
              path: 'project/:id/*',
              element: <Project />,
            },
          ],
        },
        {
          path: 'settings',
          element: <SettingLayout />,
          errorElement: <ErrorPage />,
          children: [
            { path: 'order-management', element: <OrderManagement /> },
            { path: 'my-likes', element: <MyLikes /> },
            { path: 'my-collections', element: <MyCollections /> },
            { path: 'account-bindings', element: <AccountBindings /> },
            {
              path: 'model',
              element: <ModelSettings />,
            },
            {
              path: 'profile',
              element: <ProfileLayout />,
              errorElement: <ErrorPage />,
              children: [
                { index: true, element: <ProfileMyInfo /> },
                { path: 'my-info', element: <ProfileMyInfo /> },
                { path: 'api-key', element: <ProfileApiKey /> },
                { path: 'usage', element: <ProfileUsage /> },
                { path: 'purchase', element: <ProfilePurchase /> },
              ],
            },
            {
              path: 'organization',
              element: <OrganizationManagement />,
            },
            {
              path: 'personnel',
              element: <Personnel />,
            },
            {
              path: 'tag-group',
              element: <TagGroupManagement />,
            },
            {
              path: 'tag',
              element: <TagManagement />,
            },
            {
              path: 'synonym',
              element: <SynonymManagement />,
            },
            {
              path: 'industry-term',
              element: <IndustryTermManagement />,
            },
          ],
        },
      ],
    },
    // 联系售前页面：公开路由，无需登录，刷新不会触发全局认证接口
    { path: '/version', element: <Version /> },
    {
      path: 'invite',
      element: <Invite />,
      errorElement: <ErrorPage />,
    },
    {
      path: 'iframe',
      element: <EmbedLayout />,
      errorElement: <ErrorPage />,
      children: [
        {
          path: 'detail/role/:id',
          element: <DetailRoleEmbed />,
        },
        {
          path: 'detail/prompt/:id',
          element: <DetailPromptEmbed />,
        },
        {
          path: 'detail/workflow/:id',
          element: <DetailPromptEmbed workflow />,
        },
        {
          path: 'detail/question/:id',
          element: <DetailQuestionEmbed />,
        },
        {
          path: 'widget/role/:id',
          element: <EmbedWidget />,
        },
        {
          path: 'widget/prompt/:id',
          element: <EmbedWidget />,
        },
        {
          path: 'widget/workflow/:id',
          element: <EmbedWidget workflow />,
        },
        {
          path: 'widget/question/:id',
          element: <EmbedWidget />,
        },
      ],
    },
    { path: '*', element: <ErrorPage /> },
  ],
  {
    basename: import.meta.env.BASE_URL,
  },
)

export default router
