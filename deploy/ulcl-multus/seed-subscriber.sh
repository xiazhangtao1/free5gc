#!/usr/bin/env bash
set -euo pipefail

NAMESPACE=${NAMESPACE:-free5gc}
MONGO_POD=${MONGO_POD:-mongodb-0}
IMSI=${IMSI:-460110000000100}
PLMN=${PLMN:-46011}
DNN=${DNN:-cmnet}
SST=${SST:-1}
SD=${SD:-010101}
K=${K:-12345600000000000000000000000000}
OPC=${OPC:-12345600000000000000000000000000}
SQN=${SQN:-000000000020}
AMF=${AMF:-8000}

SNSSAI_KEY=$(printf "%02x%s" "$SST" "$SD")
UE_ID="imsi-${IMSI}"

kubectl wait -n "$NAMESPACE" --for=condition=Ready "pod/${MONGO_POD}" --timeout=180s >/dev/null

kubectl exec -n "$NAMESPACE" "$MONGO_POD" -- mongosh free5gc --quiet --eval "
const ueId = '${UE_ID}';
const plmn = '${PLMN}';
const dnn = '${DNN}';
const sst = Number('${SST}');
const sd = '${SD}';
const snssaiKey = '${SNSSAI_KEY}';
const k = '${K}';
const opc = '${OPC}';
const sqn = '${SQN}';
const amf = '${AMF}';

function upsert(collection, document) {
  db.getCollection(collection).updateOne({ ueId: ueId }, { \$set: document }, { upsert: true });
}

upsert('subscriptionData.provisionedData.smfSelectionSubscriptionData', {
  ueId: ueId,
  servingPlmnId: plmn,
  subscribedSnssaiInfos: {
    [snssaiKey]: {
      dnnInfos: [{ dnn: dnn, defaultDnnIndicator: true }]
    }
  }
});

upsert('subscriptionData.provisionedData.smData', {
  ueId: ueId,
  servingPlmnId: plmn,
  singleNssai: { sst: sst, sd: sd },
  dnnConfigurations: {
    [dnn]: {
      pduSessionTypes: {
        defaultSessionType: 'IPV4',
        allowedSessionTypes: ['IPV4']
      },
      sscModes: {
        defaultSscMode: 'SSC_MODE_1',
        allowedSscModes: ['SSC_MODE_1']
      },
      '5gQosProfile': {
        '5qi': 9,
        arp: {
          priorityLevel: 8,
          preemptCap: 'NOT_PREEMPT',
          preemptVuln: 'PREEMPTABLE'
        },
        priorityLevel: 8
      },
      sessionAmbr: {
        uplink: '1000 Mbps',
        downlink: '1000 Mbps'
      }
    }
  }
});

upsert('policyData.ues.smData', {
  ueId: ueId,
  smPolicySnssaiData: {
    [snssaiKey]: {
      snssai: { sst: sst, sd: sd },
      smPolicyDnnData: {
        [dnn]: { dnn: dnn }
      }
    }
  }
});

upsert('policyData.ues.amData', {
  ueId: ueId,
  subscCats: ['free5gc']
});

upsert('subscriptionData.provisionedData.amData', {
  ueId: ueId,
  servingPlmnId: plmn,
  gpsis: ['msisdn-0900000100'],
  nssai: {
    defaultSingleNssais: [{ sst: sst, sd: sd }],
    singleNssais: [{ sst: sst, sd: sd }]
  },
  subscribedUeAmbr: {
    uplink: '1000 Mbps',
    downlink: '1000 Mbps'
  }
});

upsert('subscriptionData.authenticationData.authenticationSubscription', {
  ueId: ueId,
  authenticationMethod: '5G_AKA',
  authenticationManagementField: amf,
  permanentKey: {
    permanentKeyValue: k,
    encryptionKey: 0,
    encryptionAlgorithm: 0
  },
  sequenceNumber: sqn,
  milenage: {
    op: {
      opValue: '',
      encryptionKey: 0,
      encryptionAlgorithm: 0
    }
  },
  opc: {
    opcValue: opc,
    encryptionKey: 0,
    encryptionAlgorithm: 0
  }
});

print('Seeded default free5GC subscriber ' + ueId + ' PLMN=' + plmn + ' DNN=' + dnn + ' S-NSSAI=' + snssaiKey);
"
