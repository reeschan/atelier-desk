export namespace atelier {
	
	export class Application {
	    id: string;
	    template: string;
	    until?: string;
	    on_expire: string;
	    reason: string;
	    started_at: string;
	    started_via: string;
	
	    static createFrom(source: any = {}) {
	        return new Application(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.template = source["template"];
	        this.until = source["until"];
	        this.on_expire = source["on_expire"];
	        this.reason = source["reason"];
	        this.started_at = source["started_at"];
	        this.started_via = source["started_via"];
	    }
	}
	export class Requires {
	    skills: string[];
	    plugins: string[];
	    cli: string[];
	    env: string[];
	
	    static createFrom(source: any = {}) {
	        return new Requires(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.skills = source["skills"];
	        this.plugins = source["plugins"];
	        this.cli = source["cli"];
	        this.env = source["env"];
	    }
	}
	export class Invoke {
	    kind: string;
	    name: string;
	    plugin: string;
	    path: string;
	    args: any;
	
	    static createFrom(source: any = {}) {
	        return new Invoke(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.plugin = source["plugin"];
	        this.path = source["path"];
	        this.args = source["args"];
	    }
	}
	export class Text {
	    ja: string;
	    en: string;
	
	    static createFrom(source: any = {}) {
	        return new Text(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ja = source["ja"];
	        this.en = source["en"];
	    }
	}
	export class ComponentData {
	    id: string;
	    stages: string[];
	    title: Text;
	    invoke: Invoke;
	    doc: string;
	    where: string;
	    memory: string;
	    when: string;
	    cost: string;
	    interactive: boolean;
	    calls_back: boolean;
	    requires?: Requires;
	    forbid: string[];
	    license: string;
	    link: string;
	    source: string;
	    then: string[];
	
	    static createFrom(source: any = {}) {
	        return new ComponentData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.stages = source["stages"];
	        this.title = this.convertValues(source["title"], Text);
	        this.invoke = this.convertValues(source["invoke"], Invoke);
	        this.doc = source["doc"];
	        this.where = source["where"];
	        this.memory = source["memory"];
	        this.when = source["when"];
	        this.cost = source["cost"];
	        this.interactive = source["interactive"];
	        this.calls_back = source["calls_back"];
	        this.requires = this.convertValues(source["requires"], Requires);
	        this.forbid = source["forbid"];
	        this.license = source["license"];
	        this.link = source["link"];
	        this.source = source["source"];
	        this.then = source["then"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Component {
	    id: string;
	    source: string;
	    data: ComponentData;
	
	    static createFrom(source: any = {}) {
	        return new Component(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.data = this.convertValues(source["data"], ComponentData);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class ModelRef {
	    quality?: number;
	    price_in?: number;
	    price_out?: number;
	    sample?: boolean;
	    source?: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.quality = source["quality"];
	        this.price_in = source["price_in"];
	        this.price_out = source["price_out"];
	        this.sample = source["sample"];
	        this.source = source["source"];
	    }
	}
	export class Model {
	    id: string;
	    name: string;
	    efforts: string[];
	    ref?: ModelRef;
	    manual: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Model(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.efforts = source["efforts"];
	        this.ref = this.convertValues(source["ref"], ModelRef);
	        this.manual = source["manual"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ModelList {
	    fetched_at: string;
	    method: string;
	    error?: string;
	    failed_at?: string;
	    list: Model[];
	
	    static createFrom(source: any = {}) {
	        return new ModelList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fetched_at = source["fetched_at"];
	        this.method = source["method"];
	        this.error = source["error"];
	        this.failed_at = source["failed_at"];
	        this.list = this.convertValues(source["list"], Model);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class RoutingTable {
	    name: string;
	    tiers: Record<string, Array<Step>>;
	
	    static createFrom(source: any = {}) {
	        return new RoutingTable(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.tiers = this.convertValues(source["tiers"], Array<Step>, true);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Routing {
	    source: string;
	    table: RoutingTable;
	
	    static createFrom(source: any = {}) {
	        return new Routing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.table = this.convertValues(source["table"], RoutingTable);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Runner {
	    enabled: boolean;
	    path: string;
	    version: string;
	    checked_at: string;
	    billing: string;
	    selected: string[];
	    missing_selected: string[];
	    models?: ModelList;
	
	    static createFrom(source: any = {}) {
	        return new Runner(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.path = source["path"];
	        this.version = source["version"];
	        this.checked_at = source["checked_at"];
	        this.billing = source["billing"];
	        this.selected = source["selected"];
	        this.missing_selected = source["missing_selected"];
	        this.models = this.convertValues(source["models"], ModelList);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Template {
	    name: string;
	    description: string;
	    tiers: Record<string, Array<TemplateStep>>;
	
	    static createFrom(source: any = {}) {
	        return new Template(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.tiers = this.convertValues(source["tiers"], Array<TemplateStep>, true);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class State {
	    home: string;
	    runners: Record<string, Runner>;
	    routing: Routing;
	    templates: Record<string, Template>;
	    examples: Record<string, Template>;
	    application?: Application;
	    application_status: string;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new State(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.home = source["home"];
	        this.runners = this.convertValues(source["runners"], Runner, true);
	        this.routing = this.convertValues(source["routing"], Routing);
	        this.templates = this.convertValues(source["templates"], Template, true);
	        this.examples = this.convertValues(source["examples"], Template, true);
	        this.application = this.convertValues(source["application"], Application);
	        this.application_status = source["application_status"];
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Step {
	    runner: string;
	    model: string;
	    effort?: string;
	    until?: string;
	
	    static createFrom(source: any = {}) {
	        return new Step(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runner = source["runner"];
	        this.model = source["model"];
	        this.effort = source["effort"];
	        this.until = source["until"];
	    }
	}
	
	export class TemplateStep {
	    runner?: string;
	    model?: string;
	    effort?: string;
	    until?: string;
	    rotate?: Step[];
	
	    static createFrom(source: any = {}) {
	        return new TemplateStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runner = source["runner"];
	        this.model = source["model"];
	        this.effort = source["effort"];
	        this.until = source["until"];
	        this.rotate = this.convertValues(source["rotate"], Step);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class WorkflowEntry {
	    id: string;
	    when: string;
	    where: string;
	    models: string[];
	
	    static createFrom(source: any = {}) {
	        return new WorkflowEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.when = source["when"];
	        this.where = source["where"];
	        this.models = source["models"];
	    }
	}
	export class WorkflowStep {
	    kind: string;
	    stage: string;
	    after: string;
	    components: WorkflowEntry[];
	
	    static createFrom(source: any = {}) {
	        return new WorkflowStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.stage = source["stage"];
	        this.after = source["after"];
	        this.components = this.convertValues(source["components"], WorkflowEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Workflow {
	    name: string;
	    source: string;
	    bundled: boolean;
	    description: Text;
	    steps: WorkflowStep[];
	    data: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new Workflow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.source = source["source"];
	        this.bundled = source["bundled"];
	        this.description = this.convertValues(source["description"], Text);
	        this.steps = this.convertValues(source["steps"], WorkflowStep);
	        this.data = source["data"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace catalog {
	
	export class ComponentView {
	    component: atelier.Component;
	    skillKey: string;
	    link: string;
	    skillMissing: boolean;
	    workflows: string[];
	
	    static createFrom(source: any = {}) {
	        return new ComponentView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.component = this.convertValues(source["component"], atelier.Component);
	        this.skillKey = source["skillKey"];
	        this.link = source["link"];
	        this.skillMissing = source["skillMissing"];
	        this.workflows = source["workflows"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Usage {
	    componentId: string;
	    workflows: string[];
	
	    static createFrom(source: any = {}) {
	        return new Usage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.componentId = source["componentId"];
	        this.workflows = source["workflows"];
	    }
	}
	export class WorkflowView {
	    workflow: atelier.Workflow;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workflow = this.convertValues(source["workflow"], atelier.Workflow);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace desk {
	
	export class CasesView {
	    cases: runs.Case[];
	    warnings: runs.ReadWarning[];
	
	    static createFrom(source: any = {}) {
	        return new CasesView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cases = this.convertValues(source["cases"], runs.Case);
	        this.warnings = this.convertValues(source["warnings"], runs.ReadWarning);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Connection {
	    state: string;
	    version: string;
	    python: string;
	    pythonVersion: string;
	    home: string;
	    code: string;
	    diagnostic: string;
	    saving: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.version = source["version"];
	        this.python = source["python"];
	        this.pythonVersion = source["pythonVersion"];
	        this.home = source["home"];
	        this.code = source["code"];
	        this.diagnostic = source["diagnostic"];
	        this.saving = source["saving"];
	    }
	}
	export class HomeView {
	    state: atelier.State;
	    base: homestore.Baseline;
	
	    static createFrom(source: any = {}) {
	        return new HomeView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = this.convertValues(source["state"], atelier.State);
	        this.base = this.convertValues(source["base"], homestore.Baseline);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RunsView {
	    runs: runs.Run[];
	    warnings: runs.ReadWarning[];
	
	    static createFrom(source: any = {}) {
	        return new RunsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runs = this.convertValues(source["runs"], runs.Run);
	        this.warnings = this.convertValues(source["warnings"], runs.ReadWarning);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SaveSettingRequest {
	    kind: string;
	    name: string;
	    input: string;
	    base: homestore.Baseline;
	
	    static createFrom(source: any = {}) {
	        return new SaveSettingRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.input = source["input"];
	        this.base = this.convertValues(source["base"], homestore.Baseline);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SkillView {
	    detail: skills.SkillDetail;
	    usedBy: catalog.Usage[];
	
	    static createFrom(source: any = {}) {
	        return new SkillView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.detail = this.convertValues(source["detail"], skills.SkillDetail);
	        this.usedBy = this.convertValues(source["usedBy"], catalog.Usage);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace homestore {
	
	export class Baseline {
	    // Go type: time
	    Taken: any;
	    Files: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Baseline(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Taken = this.convertValues(source["Taken"], null);
	        this.Files = source["Files"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SaveResult {
	    Outcome: string;
	    Problems: string[];
	    Changed: string[];
	    Written: string[];
	    Failed: string;
	    BackupDir: string;
	    RecoveryFile: string;
	    Current: Baseline;
	    Base: Baseline;
	
	    static createFrom(source: any = {}) {
	        return new SaveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Outcome = source["Outcome"];
	        this.Problems = source["Problems"];
	        this.Changed = source["Changed"];
	        this.Written = source["Written"];
	        this.Failed = source["Failed"];
	        this.BackupDir = source["BackupDir"];
	        this.RecoveryFile = source["RecoveryFile"];
	        this.Current = this.convertValues(source["Current"], Baseline);
	        this.Base = this.convertValues(source["Base"], Baseline);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace runs {
	
	export class Attempt {
	    Report: string;
	    Attempt: number;
	    Rework: number;
	    Step: string;
	    Runner: string;
	    Model: string;
	    Outcome: string;
	    Causes: string[];
	    Findings: number[];
	    Verify: number[];
	    Raw: number[];
	
	    static createFrom(source: any = {}) {
	        return new Attempt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Report = source["Report"];
	        this.Attempt = source["Attempt"];
	        this.Rework = source["Rework"];
	        this.Step = source["Step"];
	        this.Runner = source["Runner"];
	        this.Model = source["Model"];
	        this.Outcome = source["Outcome"];
	        this.Causes = source["Causes"];
	        this.Findings = source["Findings"];
	        this.Verify = source["Verify"];
	        this.Raw = source["Raw"];
	    }
	}
	export class Case {
	    Repo: string;
	    WorkKey: string;
	    PBI: string;
	    Causes: string[];
	    Summary: string;
	    Archived: boolean;
	    Raw: number[];
	
	    static createFrom(source: any = {}) {
	        return new Case(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Repo = source["Repo"];
	        this.WorkKey = source["WorkKey"];
	        this.PBI = source["PBI"];
	        this.Causes = source["Causes"];
	        this.Summary = source["Summary"];
	        this.Archived = source["Archived"];
	        this.Raw = source["Raw"];
	    }
	}
	export class PBIResult {
	    ID: string;
	    Passed: boolean;
	    Reworks: number;
	    Escalations: number;
	    Held: boolean;
	    HoldReason: string;
	    Model: string;
	    Attempts: Attempt[];
	
	    static createFrom(source: any = {}) {
	        return new PBIResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Passed = source["Passed"];
	        this.Reworks = source["Reworks"];
	        this.Escalations = source["Escalations"];
	        this.Held = source["Held"];
	        this.HoldReason = source["HoldReason"];
	        this.Model = source["Model"];
	        this.Attempts = this.convertValues(source["Attempts"], Attempt);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ReadWarning {
	    Path: string;
	    Line: number;
	    Reason: string;
	
	    static createFrom(source: any = {}) {
	        return new ReadWarning(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Path = source["Path"];
	        this.Line = source["Line"];
	        this.Reason = source["Reason"];
	    }
	}
	export class Run {
	    Repo: string;
	    WorkKey: string;
	    // Go type: time
	    FirstAt: any;
	    // Go type: time
	    LastAt: any;
	    Archived: boolean;
	    Source: string;
	    PBIs: PBIResult[];
	
	    static createFrom(source: any = {}) {
	        return new Run(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Repo = source["Repo"];
	        this.WorkKey = source["WorkKey"];
	        this.FirstAt = this.convertValues(source["FirstAt"], null);
	        this.LastAt = this.convertValues(source["LastAt"], null);
	        this.Archived = source["Archived"];
	        this.Source = source["Source"];
	        this.PBIs = this.convertValues(source["PBIs"], PBIResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace skills {
	
	export class AuxFile {
	    RelPath: string;
	    Size: number;
	
	    static createFrom(source: any = {}) {
	        return new AuxFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.RelPath = source["RelPath"];
	        this.Size = source["Size"];
	    }
	}
	export class ChangelogEntry {
	    Heading: string;
	    At: string;
	    Version: string;
	    Kind: string;
	    Body: string;
	
	    static createFrom(source: any = {}) {
	        return new ChangelogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Heading = source["Heading"];
	        this.At = source["At"];
	        this.Version = source["Version"];
	        this.Kind = source["Kind"];
	        this.Body = source["Body"];
	    }
	}
	export class Skill {
	    Key: string;
	    Name: string;
	    Description: string;
	    Origin: string;
	    Plugin: string;
	    PluginVersion: string;
	    RelPath: string;
	    Dir: string;
	    SkillFile: string;
	    DisableModelInvocation: boolean;
	    Versioned: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Skill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Key = source["Key"];
	        this.Name = source["Name"];
	        this.Description = source["Description"];
	        this.Origin = source["Origin"];
	        this.Plugin = source["Plugin"];
	        this.PluginVersion = source["PluginVersion"];
	        this.RelPath = source["RelPath"];
	        this.Dir = source["Dir"];
	        this.SkillFile = source["SkillFile"];
	        this.DisableModelInvocation = source["DisableModelInvocation"];
	        this.Versioned = source["Versioned"];
	    }
	}
	export class VersionInfo {
	    Active: string;
	    Versions: string[];
	    Changelog: ChangelogEntry[];
	
	    static createFrom(source: any = {}) {
	        return new VersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Active = source["Active"];
	        this.Versions = source["Versions"];
	        this.Changelog = this.convertValues(source["Changelog"], ChangelogEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SkillDetail {
	    Skill: Skill;
	    Frontmatter: Record<string, any>;
	    Body: string;
	    Files: AuxFile[];
	    Versions?: VersionInfo;
	
	    static createFrom(source: any = {}) {
	        return new SkillDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Skill = this.convertValues(source["Skill"], Skill);
	        this.Frontmatter = source["Frontmatter"];
	        this.Body = source["Body"];
	        this.Files = this.convertValues(source["Files"], AuxFile);
	        this.Versions = this.convertValues(source["Versions"], VersionInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

