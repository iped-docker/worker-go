#!/bin/bash
set -e

VARS_LOCAL_CONFIG="
        iped_locale
        iped_indexTemp
        iped_indexTempOnSSD
        iped_outputOnSSD
        iped_numThreads
        iped_kffDb
        iped_ledWkffPath
        iped_photoDNAHashDatabase
        iped_ledDie
        iped_tskJarPath
        iped_mplayerPath
        iped_optional_jars
        iped_regripperFolder
"

VARS_IPED_CONFIG="
        iped_hash
        iped_enablePhotoDNA
        iped_enableKff
        iped_enableLedWkff
        iped_enableLedDie
        iped_excludeKffIgnorable
        iped_ignoreDuplicates
        iped_exportFileProps
        iped_processFileSignatures
        iped_enableFileParsing
        iped_expandContainers
        iped_enableRegexSearch
        iped_enableLanguageDetect
        iped_enableNamedEntityRecogniton
        iped_enableGraphGeneration
        iped_indexFileContents
        iped_indexUnknownFiles
        iped_indexCorruptedFiles
        iped_enableOCR
        iped_enableAudioTranscription
        iped_addFileSlacks
        iped_addUnallocated
        iped_indexUnallocated
        iped_enableCarving
        iped_enableKFFCarving
        iped_enableKnownMetCarving
        iped_enableImageThumbs
        iped_enableImageSimilarity
        iped_enableVideoThumbs
        iped_enableHTMLReport
"

VARS_ADVANCED_CONFIG="
        iped_robustImageReading
        iped_numImageReaders
        iped_enableExternalParsing
        iped_numExternalParsers
        iped_externalParsingMaxMem
        iped_phoneParsersToUse
        iped_forceMerge
        iped_timeOut
        iped_timeOutPerMB
        iped_embutirLibreOffice
        iped_sortPDFChars
        iped_entropyTest
        iped_minRawStringSize
        iped_extraCharsToIndex
        iped_convertCharsToLowerCase
        iped_filterNonLatinChars
        iped_convertCharsToAscii
        iped_ignoreHardLinks
        iped_minOrphanSizeToIgnore
        iped_unallocatedFragSize
        iped_minItemSizeToFragment
        iped_textSplitSize
        iped_useNIOFSDirectory
        iped_commitIntervalSeconds
        iped_OCRLanguage
        iped_pageSegMode
        iped_minFileSize2OCR
        iped_maxFileSize2OCR
        iped_pdfToImgResolution
        iped_maxPDFTextSize2OCR
        iped_pdfToImgLib
        iped_externalPdfToImgConv
        iped_externalConvMaxMem
        iped_processImagesInPDFs
        iped_searchThreads
        iped_maxBackups
        iped_backupInterval
        iped_autoManageCols
        iped_preOpenImagesOnSleuth
        iped_openImagesCacheWarmUpEnabled
        iped_openImagesCacheWarmUpThreads
"

# test all iped_* variables, searching for typos
awk 'BEGIN{for(v in ENVIRON) print v}' | grep iped_ | while read v
do
        # grep: -F fixed strings; -w entire words; -c count
        # cat is there to ignore grep's exit status
        count=$(echo "${VARS_LOCAL_CONFIG}" "${VARS_IPED_CONFIG}" "${VARS_ADVANCED_CONFIG}" | grep -Fwc "${v}" | cat)
        if [[ $count == 0 ]]
        then
                echo ${v} is not a valid variable config >&2
                echo valid options are "${VARS_LOCAL_CONFIG}" "${VARS_IPED_CONFIG}" "${VARS_ADVANCED_CONFIG}" >&2
                exit 1
        fi
done

# LocalConfig.txt variables (with iped_ prefix)
for v in ${VARS_LOCAL_CONFIG}
do
        if [ "${!v}" ]
        then
                echo ${v} = ${!v}
                sed -i -e "s|.*${v#iped_} =.*|${v#iped_} = ${!v}|" /root/IPED/iped/LocalConfig.txt
        fi
done

# IPEDConfig.txt variables (with iped_ prefix)
for v in ${VARS_IPED_CONFIG}
do
        if [ "${!v}" ]
        then
                echo ${v} = ${!v}
                sed -i -e "s|.*${v#iped_} =.*|${v#iped_} = ${!v}|" /root/IPED/iped/profiles/*/*/IPEDConfig.txt
        fi
done

# AdvancedConfig.txt variables (with iped_ prefix)
for v in ${VARS_ADVANCED_CONFIG}
do
        if [ "${!v}" ]
        then
                echo ${v} = ${!v}
                sed -i -e "s|.*${v#iped_} =.*|${v#iped_} = ${!v}|" /root/IPED/iped/profiles/*/*/conf/AdvancedConfig.txt
        fi
done

exec "$@"
